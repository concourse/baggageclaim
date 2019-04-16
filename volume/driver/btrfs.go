package driver

import (
	"bytes"
	"os/exec"

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
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "delete", path)
	if err != nil {
		return err
	}

	return nil
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "snapshot", parent, path)
	return err
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
