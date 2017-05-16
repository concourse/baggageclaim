package api

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func (vs *VolumeServer) streamIn(stream io.Reader, dest string, privileged bool) (bool, error) {
	tarCommand := exec.Command("tar", "-x", "-C", dest)
	tarCommand.Stdin = stream

	if !privileged {
		vs.namespacer.NamespaceCommand(tarCommand)
	}

	err := tarCommand.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return true, err
		}

		return false, err
	}

	return false, nil
}

func (vs *VolumeServer) streamOut(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarCommandPath, tarCommandDir string

	if fileInfo.IsDir() {
		tarCommandPath = "."
		tarCommandDir = src
	} else {
		tarCommandPath = filepath.Base(src)
		tarCommandDir = filepath.Dir(src)
	}

	tarCommand := exec.Command("tar", "-c", tarCommandPath)
	tarCommand.Dir = tarCommandDir

	if !privileged {
		vs.namespacer.NamespaceCommand(tarCommand)
	}

	readCloser, err := tarCommand.StdoutPipe()
	if err != nil {
		return err
	}

	err = tarCommand.Start()
	if err != nil {
		return err
	}

	_, err = io.Copy(w, readCloser)
	if err != nil {
		return err
	}

	return tarCommand.Wait()
}
