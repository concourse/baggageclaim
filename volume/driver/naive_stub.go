// +build !windows

package driver

import (
	"bytes"
	"fmt"
	"os/exec"
)

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return exec.Command("cp", "-rp", parent, path).Run()
}

func (driver *NaiveDriver) GetVolumeSizeInBytes(path string) (int64, error) {
	stdout := &bytes.Buffer{}
	cmd := exec.Command("du", "-s", path)
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

	return size * 512, nil
}
