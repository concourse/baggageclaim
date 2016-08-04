package volume

import (
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

type ImportStrategy struct {
	Path string
}

func (strategy ImportStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	initVolume, err := fs.NewVolume(handle)
	if err != nil {
		return nil, err
	}

	destination := initVolume.DataPath()

	cmd := exec.Command("cp", "-a", filepath.Clean(strategy.Path)+"/.", destination)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	return initVolume, nil
}
