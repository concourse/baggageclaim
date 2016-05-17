package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

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
	_, _, err := driver.run("btrfs", "subvolume", "create", path)
	return err
}

func (driver *BtrFSDriver) DestroyVolume(path string) error {
	_, _, err := driver.run("btrfs", "subvolume", "delete", path)
	return err
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, _, err := driver.run("btrfs", "subvolume", "snapshot", parent, path)
	return err
}

func (driver *BtrFSDriver) GetVolumeSize(path string) (uint, error) {
	_, _, err := driver.run("btrfs", "quota", "enable", path)
	if err != nil {
		return 0, err
	}

	output, _, err := driver.run("btrfs", "qgroup", "show", "-F", "--raw", path)
	if err != nil {
		return 0, err
	}

	qgroupsLines := strings.Split(strings.TrimSpace(output), "\n")
	qgroupFields := strings.Fields(qgroupsLines[len(qgroupsLines)-1])

	if len(qgroupFields) != 3 {
		return 0, errors.New("unable-to-parse-btrfs-qgroup-show")
	}

	var exclusiveSize uint
	_, err = fmt.Sscanf(qgroupFields[2], "%d", &exclusiveSize)

	if err != nil {
		return 0, err
	}

	return exclusiveSize, nil
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
