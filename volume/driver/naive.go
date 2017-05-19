package driver

import (
	"os"
)

type NaiveDriver struct{}

func (driver *NaiveDriver) CreateVolume(path string) error {
	return os.Mkdir(path, 0755)
}

func (driver *NaiveDriver) DestroyVolume(path string) error {
	return os.RemoveAll(path)
}
