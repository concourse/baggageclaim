package fs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pivotal-golang/lager"
)

type BtrfsFilesystem struct {
	imagePath string
	mountPath string

	logger lager.Logger
}

func New(logger lager.Logger, imagePath string, mountPath string) *BtrfsFilesystem {
	return &BtrfsFilesystem{
		imagePath: imagePath,
		mountPath: mountPath,
		logger:    logger,
	}
}

func (fs *BtrfsFilesystem) Create(bytes uint64) error {
	kiloBytes := bytes / 1024

	_, err := fs.run(
		"dd",
		"if=/dev/zero",
		fmt.Sprintf("of=%s", fs.imagePath),
		"bs=1024",
		fmt.Sprintf("count=%d", kiloBytes),
	)
	if err != nil {
		return err
	}

	output, err := fs.run(
		"losetup",
		"-f",
		"--show",
		fs.imagePath,
	)
	if err != nil {
		return err
	}

	loopbackDevice := strings.TrimSpace(string(output))

	_, err = fs.run(
		"mkfs.btrfs",
		loopbackDevice,
	)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(fs.mountPath, 0755); err != nil {
		return err
	}

	_, err = fs.run(
		"mount",
		loopbackDevice,
		fs.mountPath,
	)
	if err != nil {
		return err
	}

	return nil
}

func (fs *BtrfsFilesystem) Delete() error {
	_, err := fs.run(
		"umount",
		fs.mountPath,
	)
	if err != nil {
		return err
	}

	if err := os.RemoveAll(fs.mountPath); err != nil {
		return err
	}

	loopbackOutput, err := fs.run(
		"losetup",
		"-j",
		fs.imagePath,
	)
	if err != nil {
		return err
	}

	loopbackDevice := strings.Split(loopbackOutput, ":")[0]

	_, err = fs.run(
		"losetup",
		"-d",
		loopbackDevice,
	)
	if err != nil {
		return err
	}

	return os.Remove(fs.imagePath)
}

func (fs *BtrfsFilesystem) run(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	logger := fs.logger.Session("run-command", lager.Data{
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
