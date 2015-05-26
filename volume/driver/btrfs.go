package driver

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"syscall"

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

func (driver *BtrFSDriver) Setup(logger lager.Logger, rootPath string, sizeInBytes uint64) (string, error) {
	imagePath := filepath.Join(rootPath, "image.img")
	mountPoint := filepath.Join(rootPath, "mountpoint")

	driver.fs = fs.New(
		logger.Session("filesystem"),
		imagePath,
		mountPoint,
	)
	err := driver.fs.Create(sizeInBytes)
	if err != nil {
		return "", err
	}

	return mountPoint, nil
}

func (driver *BtrFSDriver) Teardown() error {
	return driver.fs.Delete()
}

func (driver *BtrFSDriver) CreateVolume(path string) error {
	_, err := driver.run("btrfs", "subvolume", "create", path)
	return err
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, err := driver.run("btrfs", "subvolume", "snapshot", parent, path)
	return err
}

func (driver *BtrFSDriver) run(command string, args ...string) (string, error) {
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
		"status": cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(),
	}

	if err != nil {
		logger.Error("failed", err, loggerData)
		return "", err
	}

	logger.Debug("ran", loggerData)

	return stdout.String(), nil
}
