package volume

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func (repo *repository) streamIn(stream io.Reader, dest string, privileged bool) (bool, error) {
	tarCommand := exec.Command("tar", "-x", "-C", dest)
	tarCommand.Stdin = stream

	repo.namespacer(privileged).NamespaceCommand(tarCommand)

	err := tarCommand.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return true, err
		}

		return false, err
	}

	return false, nil
}

func (repo *repository) streamOut(w io.Writer, src string, privileged bool) error {
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
	tarCommand.Stdout = w

	repo.namespacer(privileged).NamespaceCommand(tarCommand)

	err = tarCommand.Run()
	if err != nil {
		return err
	}

	return nil
}
