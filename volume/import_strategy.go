package volume

import (
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

type ImportStrategy struct {
	Path           string
	FollowSymlinks bool
}

func (strategy ImportStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	initVolume, err := fs.NewVolume(handle)
	if err != nil {
		return nil, err
	}

	destination := initVolume.DataPath()

	cpFlags := "-a"
	if strategy.FollowSymlinks {
		cpFlags = "-Lr"
	}

	cmd := exec.Command("cp", cpFlags, filepath.Clean(strategy.Path)+"/.", destination)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	return initVolume, nil
}
