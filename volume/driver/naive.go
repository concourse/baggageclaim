package driver

import (
	"os"
	"os/exec"
)

type NaiveDriver struct{}

func (driver *NaiveDriver) CreateVolume(path string) error {
	return os.MkdirAll(path, 0755)
}

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return exec.Command("cp", "-r", parent, path).Run()
}
