package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/lager"
)

type BtrFSDriver struct {
	btrfsBin string
	rootPath string
	logger   lager.Logger
}

func NewBtrFSDriver(
	logger lager.Logger,
	rootPath string,
	btrfsBin string,
) *BtrFSDriver {
	return &BtrFSDriver{
		logger:   logger,
		rootPath: rootPath,
		btrfsBin: btrfsBin,
	}
}

func (driver *BtrFSDriver) CreateVolume(path string) error {
	_, _, err := driver.run(driver.btrfsBin, "subvolume", "create", path)
	if err != nil {
		return err
	}

	_, _, err = driver.run(driver.btrfsBin, "quota", "enable", path)
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
	output, _, err := driver.run(driver.btrfsBin, "qgroup", "show", "-F", "--raw", path)
	if err != nil {
		return 0, err
	}

	qgroupsLines := strings.Split(strings.TrimSpace(output), "\n")
	qgroupFields := strings.Fields(qgroupsLines[len(qgroupsLines)-1])

	if len(qgroupFields) != 3 {
		return 0, errors.New("unable-to-parse-btrfs-qgroup-show")
	}

	var exclusiveSize int64
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
