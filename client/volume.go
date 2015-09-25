package client

import (
	"sync"
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/pivotal-golang/lager"
)

type clientVolume struct {
	handle string
	path   string

	bcClient *client

	releaseOnce      sync.Once
	heartbeating     *sync.WaitGroup
	stopHeartbeating chan interface{}
}

func (client *client) newVolume(handle string, path string) baggageclaim.Volume {
	return &clientVolume{
		handle: handle,
		path:   path,

		bcClient:         client,
		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan interface{}),
	}
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
}

func (cv *clientVolume) Properties() (baggageclaim.VolumeProperties, error) {
	vr, err := cv.bcClient.getVolumeResponse(cv.handle)
	if err != nil {
		return nil, err
	}

	return vr.Properties, nil
}

func (cv *clientVolume) Expiration() (uint, time.Time, error) {
	vr, err := cv.bcClient.getVolumeResponse(cv.handle)
	if err != nil {
		return 0, time.Time{}, err
	}

	return vr.TTL, vr.ExpiresAt, nil
}

func (cv *clientVolume) SetTTL(timeInSeconds uint) error {
	return cv.bcClient.setTTL(cv.handle, timeInSeconds)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.handle, name, value)
}

func (cv *clientVolume) Heartbeat(logger lager.Logger, ttlInSeconds uint) {
	interval := (time.Duration(ttlInSeconds) * time.Second) / 2

	cv.heartbeating.Add(1)
	go cv.heartbeat(logger.Session("heartbeating"), ttlInSeconds, time.NewTicker(interval))

	return
}

func (cv *clientVolume) Release() {
	cv.releaseOnce.Do(func() {
		close(cv.stopHeartbeating)
		cv.heartbeating.Wait()
	})

	return
}

func (cv *clientVolume) heartbeat(logger lager.Logger, ttlInSeconds uint, pacemaker *time.Ticker) {
	defer cv.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	if !cv.heartbeatTick(logger.Session("initial-heartbeat"), ttlInSeconds) {
		return
	}

	for {
		select {
		case <-pacemaker.C:
			if !cv.heartbeatTick(logger.Session("tick"), ttlInSeconds) {
				return
			}

		case <-cv.stopHeartbeating:
			return
		}
	}
}

func (cv *clientVolume) heartbeatTick(logger lager.Logger, ttlInSeconds uint) bool {
	logger.Debug("start")

	err := cv.SetTTL(ttlInSeconds)
	if err == baggageclaim.ErrVolumeNotFound {
		logger.Info("volume-disappeared")
		return false
	}

	if err != nil {
		logger.Error("failed", err)
	} else {
		logger.Debug("done")
	}

	return true
}
