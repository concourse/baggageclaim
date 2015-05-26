package driver

import (
	"os"
	"os/exec"
)

type NaiveDriver struct{}

func (driver *NaiveDriver) Setup(rootPath string) (string, error) {
	return rootPath, nil
}

func (driver *NaiveDriver) CreateVolume(path string) error {
	return os.Mkdir(path, 0755)
}

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return exec.Command("cp", "-r", parent, path).Run()
}
