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

	releaseOnce  sync.Once
	heartbeating *sync.WaitGroup
	release      chan uint
}

func (client *client) newVolume(logger lager.Logger, apiVolume baggageclaim.VolumeResponse) (baggageclaim.Volume, bool) {
	volume := &clientVolume{
		handle: apiVolume.Handle,
		path:   apiVolume.Path,

		bcClient:     client,
		heartbeating: new(sync.WaitGroup),
		release:      make(chan uint, 1),
	}

	initialHeartbeatSuccess := volume.startHeartbeating(logger, apiVolume.TTL)

	return volume, initialHeartbeatSuccess
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

func (cv *clientVolume) Release(finalTTL uint) {
	cv.releaseOnce.Do(func() {
		cv.release <- finalTTL
		cv.heartbeating.Wait()
	})

	return
}

func IntervalForTTL(ttlInSeconds uint) time.Duration {
	interval := (time.Duration(ttlInSeconds) * time.Second) / 2

	if interval > time.Minute {
		interval = time.Minute
	}

	return interval
}

func (cv *clientVolume) startHeartbeating(logger lager.Logger, ttlInSeconds uint) bool {
	if ttlInSeconds == 0 {
		return true
	}

	interval := IntervalForTTL(ttlInSeconds)

	if !cv.heartbeatTick(logger.Session("initial-heartbeat"), ttlInSeconds) {
		return false
	}

	cv.heartbeating.Add(1)
	go cv.heartbeat(logger.Session("heartbeating"), ttlInSeconds, time.NewTicker(interval))

	return true
}

func (cv *clientVolume) heartbeat(logger lager.Logger, ttlInSeconds uint, pacemaker *time.Ticker) {
	defer cv.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	for {
		select {
		case <-pacemaker.C:
			if !cv.heartbeatTick(logger.Session("tick"), ttlInSeconds) {
				return
			}

		case finalTTL := <-cv.release:
			if finalTTL != 0 {
				cv.heartbeatTick(logger.Session("final"), finalTTL)
			}

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
