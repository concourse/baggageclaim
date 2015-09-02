package driver

import (
	"bytes"
	"os/exec"

	"github.com/concourse/baggageclaim/fs"
	"github.com/pivotal-golang/lager"
)

type BtrFSDriver struct {
	fs *fs.BtrfsFilesystem

	logger lager.Logger
}

func NewBtrFSDriver(logger lager.Logger) *BtrFSDriver {
	return &BtrFSDriver{
		logger: logger,
	}
}

func (driver *BtrFSDriver) CreateVolume(path string) error {
	return driver.run("btrfs", "subvolume", "create", path)
}

func (driver *BtrFSDriver) DestroyVolume(path string) error {
	return driver.run("btrfs", "subvolume", "delete", path)
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return driver.run("btrfs", "subvolume", "snapshot", parent, path)
}

func (driver *BtrFSDriver) run(command string, args ...string) error {
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
		return err
	}

	logger.Debug("ran", loggerData)

	return nil
}
