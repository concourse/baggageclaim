package volume

import (
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

	err = cp(strategy.FollowSymlinks, filepath.Clean(strategy.Path), destination)
	if err != nil {
		return nil, err
	}

	return initVolume, nil
}
