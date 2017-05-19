package volume

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func (repo *repository) streamIn(stream io.Reader, dest string, privileged bool) (bool, error) {
	tarCommand, dirFd, err := repo.tarIn(privileged, dest, "-x")
	if err != nil {
		return false, err
	}

	defer dirFd.Close()

	tarCommand.Stdin = stream
	tarCommand.Stdout = os.Stderr
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
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

	tarCommand, dirFd, err := repo.tarIn(privileged, tarCommandDir, "-c", tarCommandPath)
	if err != nil {
		return err
	}

	defer dirFd.Close()

	tarCommand.Stdout = w
	tarCommand.Stderr = os.Stderr

	err = tarCommand.Run()
	if err != nil {
		return err
	}

	return nil
}

func (repo *repository) tarIn(privileged bool, dir string, args ...string) (*exec.Cmd, *os.File, error) {
	// 'tar' may run as MAX_UID in order to remap UIDs when streaming into an
	// unprivileged volume. this may cause permission issues when exec'ing as it
	// may not be able to even see the destination directory as non-root.
	//
	// so, open the directory while we're root, and pass it as a fd to the
	// process.
	dirFd, err := os.Open(dir)
	if err != nil {
		return nil, nil, err
	}

	tarCommand := exec.Command("tar", append([]string{"-C", "/dev/fd/3"}, args...)...)
	tarCommand.ExtraFiles = []*os.File{dirFd}

	if !privileged {
		repo.namespacer(false).NamespaceCommand(tarCommand)
	}

	return tarCommand, dirFd, nil
}
