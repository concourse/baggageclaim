package client

import (
	"sync"
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
)

type clientVolume struct {
	handle string
	path   string

	bcClient *client

	releaseOnce  sync.Once
	heartbeating *sync.WaitGroup
	release      chan time.Duration
}

func (client *client) newVolume(logger lager.Logger, apiVolume baggageclaim.VolumeResponse) (baggageclaim.Volume, bool) {
	volume := &clientVolume{
		handle: apiVolume.Handle,
		path:   apiVolume.Path,

		bcClient:     client,
		heartbeating: new(sync.WaitGroup),
		release:      make(chan time.Duration, 1),
	}

	initialHeartbeatSuccess := volume.startHeartbeating(logger, time.Duration(apiVolume.TTLInSeconds)*time.Second)

	return volume, initialHeartbeatSuccess
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
}

func (cv *clientVolume) Properties() (baggageclaim.VolumeProperties, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(cv.handle)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, volume.ErrVolumeDoesNotExist
	}

	return vr.Properties, nil
}

func (cv *clientVolume) Expiration() (time.Duration, time.Time, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(cv.handle)
	if err != nil {
		return 0, time.Time{}, err
	}
	if !found {
		return 0, time.Time{}, volume.ErrVolumeDoesNotExist
	}

	return time.Duration(vr.TTLInSeconds) * time.Second, vr.ExpiresAt, nil
}

func (cv *clientVolume) SetTTL(ttl time.Duration) error {
	return cv.bcClient.setTTL(cv.handle, ttl)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.handle, name, value)
}

func (cv *clientVolume) Release(finalTTL time.Duration) {
	cv.releaseOnce.Do(func() {
		cv.release <- finalTTL
		cv.heartbeating.Wait()
	})

	return
}

func IntervalForTTL(ttl time.Duration) time.Duration {
	interval := ttl / 2

	if interval > time.Minute {
		interval = time.Minute
	}

	return interval
}

func (cv *clientVolume) startHeartbeating(logger lager.Logger, ttl time.Duration) bool {
	if ttl == 0 {
		return true
	}

	interval := IntervalForTTL(ttl)

	if !cv.heartbeatTick(logger.Session("initial-heartbeat"), ttl) {
		return false
	}

	cv.heartbeating.Add(1)
	go cv.heartbeat(logger.Session("heartbeating"), ttl, time.NewTicker(interval))

	return true
}

func (cv *clientVolume) heartbeat(logger lager.Logger, ttl time.Duration, pacemaker *time.Ticker) {
	defer cv.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	for {
		select {
		case <-pacemaker.C:
			if !cv.heartbeatTick(logger.Session("tick"), ttl) {
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

func (cv *clientVolume) heartbeatTick(logger lager.Logger, ttl time.Duration) bool {
	logger.Debug("start")

	err := cv.SetTTL(ttl)
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
