package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type NaiveDriver struct{}

func (driver *NaiveDriver) CreateVolume(path string) error {
	return os.Mkdir(path, 0755)
}

func (driver *NaiveDriver) DestroyVolume(path string) error {
	return os.RemoveAll(path)
}

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return exec.Command("cp", "-r", parent, path).Run()
}

func (driver *NaiveDriver) GetVolumeSizeInBytes(path string) (int64, error) {
	stdout := &bytes.Buffer{}
	cmd := exec.Command("du", path)
	cmd.Stdout = stdout

	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	var size int64
	_, err = fmt.Sscanf(stdout.String(), "%d", &size)
	if err != nil {
		return 0, err
	}

	return size, nil
}
