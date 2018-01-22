package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

type BtrFSDriver struct {
	logger   lager.Logger
	btrfsBin string
}

func NewBtrFSDriver(
	logger lager.Logger,
	btrfsBin string,
) *BtrFSDriver {
	return &BtrFSDriver{
		logger:   logger,
		btrfsBin: btrfsBin,
	}
}

func (driver *BtrFSDriver) CreateVolume(path string) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "create", path)
	if err != nil {
		return err
	}

	return nil
}

func (driver *BtrFSDriver) DestroyVolume(path string) error {
	volumePathsToDelete := []string{}

	findSubvolumes := func(p string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !f.IsDir() {
			return nil
		}

		isSub, err := isSubvolume(p)
		if err != nil {
			return fmt.Errorf("failed to check if %s is a subvolume: %s", p, err)
		}

		if isSub {
			volumePathsToDelete = append(volumePathsToDelete, p)
		}

		return nil
	}

	if err := filepath.Walk(path, findSubvolumes); err != nil {
		return fmt.Errorf("recursively walking subvolumes for %s failed: %v", path, err)
	}

	for i := len(volumePathsToDelete) - 1; i >= 0; i-- {
		_, _, err := driver.run(driver.btrfsBin, "subvolume", "delete", volumePathsToDelete[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "snapshot", parent, path)
	return err
}

func (driver *BtrFSDriver) GetVolumeSizeInBytes(path string) (int64, error) {
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

func (driver *BtrFSDriver) run(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)

	logger := driver.logger.Session("run-command", lager.Data{
		"command": command,
		"args":    args,
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	loggerData := lager.Data{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		logger.Error("failed", err, loggerData)
		return "", "", err
	}

	logger.Debug("ran", loggerData)

	return stdout.String(), stderr.String(), nil
}
