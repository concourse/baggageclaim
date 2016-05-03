package driver

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
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

func (driver *NaiveDriver) GetVolumeSize(path string) (uint64, error) {
	stdout := &bytes.Buffer{}
	cmd := exec.Command("du", path)
	cmd.Stdout = stdout

	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	fields := bytes.Fields(stdout.Bytes())
	size := string(fields[0])

	return strconv.ParseUint(size, 10, 64)
}
