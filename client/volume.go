package client

import (
	"io"
	"sync"
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
)

type clientVolume struct {
	// TODO: this would be much better off as an arg to each method
	logger lager.Logger

	handle string
	path   string

	bcClient *client

	releaseOnce  sync.Once
	heartbeating *sync.WaitGroup
	release      chan *time.Duration
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
}

func (cv *clientVolume) Size() (uint, error) {
	stats, err := cv.bcClient.getVolumeStatsResponse(cv.logger, cv.handle)
	if err != nil {
		return 0, err
	}

	return stats.Size, nil
}

func (cv *clientVolume) Properties() (baggageclaim.VolumeProperties, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(cv.logger, cv.handle)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, volume.ErrVolumeDoesNotExist
	}

	return vr.Properties, nil
}

func (cv *clientVolume) Expiration() (time.Duration, time.Time, error) {
	vr, found, err := cv.bcClient.getVolumeResponse(cv.logger, cv.handle)
	if err != nil {
		return 0, time.Time{}, err
	}
	if !found {
		return 0, time.Time{}, volume.ErrVolumeDoesNotExist
	}

	return time.Duration(vr.TTLInSeconds) * time.Second, vr.ExpiresAt, nil
}

func (cv *clientVolume) StreamIn(path string, tarStream io.Reader) error {
	return cv.bcClient.streamIn(cv.logger, cv.handle, path, tarStream)
}

func (cv *clientVolume) SetTTL(ttl time.Duration) error {
	return cv.bcClient.setTTL(cv.logger, cv.handle, ttl)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.logger, cv.handle, name, value)
}

func (cv *clientVolume) Release(finalTTL *time.Duration) {
	cv.releaseOnce.Do(func() {
		cv.release <- finalTTL
		cv.heartbeating.Wait()
	})

	return
}

func IntervalForTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return time.Minute
	}

	interval := ttl / 2

	if interval > time.Minute {
		interval = time.Minute
	}

	return interval
}

func (cv *clientVolume) startHeartbeating(logger lager.Logger, ttl time.Duration) bool {
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
			if finalTTL != nil {
				cv.heartbeatTick(logger.Session("final"), *finalTTL)
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
