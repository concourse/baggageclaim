package client

import (
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
)

type clientVolume struct {
	// TODO: this would be much better off as an arg to each method
	logger lager.Logger

	handle string
	path   string

	bcClient *client

	release chan *time.Duration
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
}

func (cv *clientVolume) SizeInBytes() (int64, error) {
	stats, err := cv.bcClient.getVolumeStatsResponse(cv.logger, cv.handle)
	if err != nil {
		return 0, err
	}

	return stats.SizeInBytes, nil
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

func (cv *clientVolume) StreamOut(path string) (io.ReadCloser, error) {
	return cv.bcClient.streamOut(cv.logger, cv.handle, path)
}

func (cv *clientVolume) SetTTL(ttl time.Duration) error {
	return cv.bcClient.setTTL(cv.logger, cv.handle, ttl)
}

func (cv *clientVolume) Destroy() error {
	return cv.bcClient.destroy(cv.logger, cv.handle)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.logger, cv.handle, name, value)
}
