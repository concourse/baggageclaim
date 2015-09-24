package client

import (
	"sync"
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type clientVolume struct {
	repoVolume volume.Volume

	bcClient baggageclaim.Client

	releaseOnce      sync.Once
	heartbeating     *sync.WaitGroup
	stopHeartbeating chan interface{}
}

func NewVolume(c baggageclaim.Client, v volume.Volume) baggageclaim.Volume {
	return &clientVolume{
		repoVolume:       v,
		bcClient:         c,
		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan interface{}),
	}
}

func NewVolumes(c baggageclaim.Client, vs volume.Volumes) baggageclaim.Volumes {
	var vols baggageclaim.Volumes

	for _, v := range vs {
		vols = append(vols, NewVolume(c, v))
	}

	return vols
}

func (cv *clientVolume) Handle() string {
	return cv.repoVolume.Handle
}

func (cv *clientVolume) Path() string {
	return cv.repoVolume.Path
}

func (cv *clientVolume) Properties() volume.Properties {
	return cv.repoVolume.Properties
}

func (cv *clientVolume) TTL() uint {
	return uint(cv.repoVolume.TTL)
}

func (cv *clientVolume) ExpiresAt() time.Time {
	return cv.repoVolume.ExpiresAt
}

func (cv *clientVolume) Heartbeat(logger lager.Logger, interval time.Duration, clock clock.Clock) {
	cv.heartbeating.Add(1)
	go cv.heartbeat(logger.Session("heartbeating"), clock.NewTicker(interval))

	return
}

func (cv *clientVolume) Release() {
	cv.releaseOnce.Do(func() {
		close(cv.stopHeartbeating)
		cv.heartbeating.Wait()
	})

	return
}

func (cv *clientVolume) heartbeat(logger lager.Logger, pacemaker clock.Ticker) {
	defer cv.heartbeating.Done()
	defer pacemaker.Stop()

	logger.Debug("start")
	defer logger.Debug("done")

	if !cv.heartbeatTick(logger.Session("initial-heartbeat")) {
		return
	}

	for {
		select {
		case <-pacemaker.C():
			if !cv.heartbeatTick(logger.Session("tick")) {
				return
			}

		case <-cv.stopHeartbeating:
			return
		}
	}
}

func (cv *clientVolume) heartbeatTick(logger lager.Logger) bool {
	logger.Debug("start")

	err := cv.bcClient.SetTTL(cv.Handle(), uint(cv.repoVolume.TTL))
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
