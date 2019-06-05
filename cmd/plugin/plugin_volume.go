package main

import (
	"io"

	"github.com/concourse/baggageclaim"
)

type PluginVolume struct {
	handle string
	path   string
}

func NewPluginVolume(path, handle string) PluginVolume {
	return PluginVolume{
		handle: handle,
		path:   path,
	}
}

func (pv PluginVolume) Handle() string {
	return pv.handle
}

func (pv PluginVolume) Path() string {
	return pv.path
}

func (cv PluginVolume) Properties() (baggageclaim.VolumeProperties, error) {
	return nil, nil
}

func (cv PluginVolume) StreamIn(path string, tarStream io.Reader) error {
	return nil
}

func (cv PluginVolume) StreamOut(path string) (io.ReadCloser, error) {
	return nil, nil
}

func (cv PluginVolume) GetPrivileged() (bool, error) {
	return false, nil
}

func (cv PluginVolume) SetPrivileged(privileged bool) error {
	return nil
}

func (cv PluginVolume) Destroy() error {
	return nil
}

func (cv PluginVolume) SetProperty(name string, value string) error {
	return nil
}
