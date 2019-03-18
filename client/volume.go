package client

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"io"
)

type clientVolume struct {
	// TODO: this would be much better off as an arg to each method
	logger lager.Logger

	handle string
	path   string

	bcClient *client
}

func (cv *clientVolume) Handle() string {
	return cv.handle
}

func (cv *clientVolume) Path() string {
	return cv.path
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

func (cv *clientVolume) StreamIn(path string, tarStream io.Reader) error {
	return cv.bcClient.streamIn(cv.logger, cv.handle, path, tarStream)
}

func (cv *clientVolume) StreamOut(path string) (io.ReadCloser, error) {
	return cv.bcClient.streamOut(cv.logger, cv.handle, path)
}

func (cv *clientVolume) SetPrivileged(privileged bool) error {
	return cv.bcClient.setPrivileged(cv.logger, cv.handle, privileged)
}

func (cv *clientVolume) Destroy() error {
	return cv.bcClient.destroy(cv.logger, cv.handle)
}

func (cv *clientVolume) SetProperty(name string, value string) error {
	return cv.bcClient.setProperty(cv.logger, cv.handle, name, value)
}

