package main

import (
	"io"
	"time"

	"github.com/concourse/baggageclaim"
)

type CantTellYouNothingVolume struct {
	handle string
	path   string
}

func NewCantTellYouNothingVolume(handle, path string) CantTellYouNothingVolume {
	return CantTellYouNothingVolume{
		handle: handle,
		path:   path,
	}
}

func (pv CantTellYouNothingVolume) Handle() string {
	return pv.handle
}

func (pv CantTellYouNothingVolume) Path() string {
	return pv.path
}

func (cv CantTellYouNothingVolume) Properties() (baggageclaim.VolumeProperties, error) {
	return nil, nil
}

func (cv CantTellYouNothingVolume) StreamIn(path string, tarStream io.Reader) error {
	return nil
}

func (cv CantTellYouNothingVolume) StreamOut(path string) (io.ReadCloser, error) {
	return nil, nil
}

func (cv CantTellYouNothingVolume) SetPrivileged(privileged bool) error {
	return nil
}

func (cv CantTellYouNothingVolume) Destroy() error {
	return nil
}
func (cv CantTellYouNothingVolume) Release(duration *time.Duration) {
}

func (cv CantTellYouNothingVolume) Expiration() (time.Duration, time.Time, error) {
	return 0 * time.Second, time.Time{}, nil
}
func (cv CantTellYouNothingVolume) SetTTL(time.Duration) error {
	return nil
}
func (cv CantTellYouNothingVolume) SetProperty(name string, value string) error {
	return nil
}
