package driver

import (
	"bytes"
	"os/exec"
	"errors"
	"strconv"
	"syscall"
	"regexp"
)

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	_, err := robocopy("/e", "/nfl", "/ndl", parent, path)
	return err
}

func (driver *NaiveDriver) GetVolumeSizeInBytes(path string) (int64, error) {
	re := regexp.MustCompile("Bytes :\\s*(\\d*)")
	stdout, err := robocopy("/l", "/nfl", "/ndl", path, "\\\\localhost\\C$\\nul", "/e", "/bytes")

	if err != nil {
		return 0, err
	}

	matches := re.FindStringSubmatch(stdout)

	if matches != nil {
		size, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return size, nil
	}
	return 0, errors.New("Unable to extract size from robocopy output")
}

func robocopy(args ...string) (string, error) {
	stdout := &bytes.Buffer{}

	cmd := exec.Command("robocopy", args...)
	cmd.Stdout = stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Robocopy returns a status code indicating what action occurred. 0 means nothing was copied,
	// 1 means that files were copied successfully. Google for additional error codes.
	if err := cmd.Wait(); err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() > 1 {
					return "", err
				}
			}
		} else {
			return "", err
		}
	}

	return stdout.String(), nil
}
