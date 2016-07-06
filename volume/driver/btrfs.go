package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/concourse/baggageclaim/fs"
	"github.com/pivotal-golang/lager"
)

type BtrFSDriver struct {
	fs *fs.BtrfsFilesystem

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

type volumeParentIDs []int

func (p volumeParentIDs) Contains(id int) bool {
	for _, i := range p {
		if i == id {
			return true
		}
	}

	return false
}

func (driver *BtrFSDriver) DestroyVolume(path string) error {
	output, _, err := driver.run(driver.btrfsBin, "subvolume", "list", "--sort=path", driver.rootPath)
	parentSubPath, err := filepath.Rel(driver.rootPath, path)
	if err != nil {
		return err
	}

	volumePathsToDelete := []string{}
	format := "ID %d gen %d top level %d path %s"
	parentIDs := volumeParentIDs{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		var id int
		var gen int
		var parentID int
		var subPath string

		fmt.Sscanf(line, format, &id, &gen, &parentID, &subPath)

		if subPath == parentSubPath {
			parentIDs = append(parentIDs, id)
			volumePathsToDelete = append(volumePathsToDelete, filepath.Join(driver.rootPath, subPath))
			continue
		}

		if len(parentIDs) > 0 {
			if parentIDs.Contains(parentID) {
				parentIDs = append(parentIDs, id)
				volumePathsToDelete = append(volumePathsToDelete, filepath.Join(driver.rootPath, subPath))
				continue
			}
			break
		}
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
