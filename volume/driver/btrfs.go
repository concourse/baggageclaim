package driver

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"time"

	"code.cloudfoundry.org/lager"
)

type BtrFSDriver struct {
	btrfsBin string
	rootPath string
	logger   lager.Logger
	timeOut  time.Duration
}

var killTimeOut = 1 * time.Minute

func NewBtrFSDriver(
	logger lager.Logger,
	rootPath string,
	btrfsBin string,
	timeOut time.Duration,
) *BtrFSDriver {
	return &BtrFSDriver{
		logger:   logger,
		rootPath: rootPath,
		btrfsBin: btrfsBin,
		timeOut:  timeOut,
	}
}

func (driver *BtrFSDriver) CreateVolume(path string) error {
	_, _, err := driver.Run(driver.btrfsBin, "subvolume", "create", path)
	if err != nil {
		return err
	}

	_, _, err = driver.Run(driver.btrfsBin, "quota", "enable", path)
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
		_, _, err := driver.Run(driver.btrfsBin, "subvolume", "delete", volumePathsToDelete[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func (driver *BtrFSDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, _, err := driver.Run(driver.btrfsBin, "subvolume", "snapshot", parent, path)
	return err
}

func (driver *BtrFSDriver) GetVolumeSizeInBytes(path string) (int64, error) {
	output, _, err := driver.Run(driver.btrfsBin, "qgroup", "show", "-F", "--raw", path)
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

func (driver *BtrFSDriver) Run(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)

	logger := driver.logger.Session("run-command", lager.Data{
		"command": command,
		"args":    args,
	})

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	done := make(chan error, 1)
	go func() {
		logger.Debug("starting")
		done <- cmd.Run()
	}()

	select {
	case <-time.After(driver.timeOut):
		logger = logger.Session("timeout")
		logger.Info("attempting-to-kill-process")

		kill := make(chan error, 1)

		// btrfs has kernel bugs which can prevent commands from being killable.
		// https://bugzilla.kernel.org/show_bug.cgi?id=110391
		// We should attempt to kill this process, but do it in a non-blocking
		// fashion. If `Kill()` blocks forever, we *will* leak a go-routine.
		// But, there's nothing we can do about that.
		go func() {
			kill <- cmd.Process.Kill()
			logger.Debug("killed-process")
		}()

		select {
		case <-time.After(killTimeOut):
			err := errors.New("btrfs-kernel-error")
			logger.Error("fatal-error-timed-out-attempting-to-kill-process-restart-workers", err)

			return "", "", err
		case err := <-kill:
			if err != nil {
				logger.Error("failed-to-kill-process", err)
			}

			return stdout.String(), stderr.String(), err
		}
	case err := <-done:
		loggerData := lager.Data{
			"stdout": stdout.String(),
			"stderr": stderr.String(),
		}

		if err != nil {
			logger.Error("process-failed", err, loggerData)
			return "", "", err
		}

		logger.Debug("finished")
	}

	return stdout.String(), stderr.String(), nil
}
