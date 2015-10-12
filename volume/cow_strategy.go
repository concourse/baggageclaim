package volume

import (
	"errors"

	"github.com/pivotal-golang/lager"
)

var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

type CowStrategy struct {
	ParentHandle string
}

func (strategy CowStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	if strategy.ParentHandle == "" {
		logger.Info("parent-not-specified")
		return nil, ErrNoParentVolumeProvided
	}

	parentVolume, found, err := fs.LookupVolume(strategy.ParentHandle)
	if err != nil {
		logger.Error("failed-to-lookup-parent", err)
		return nil, err
	}

	if !found {
		logger.Info("parent-not-found")
		return nil, ErrParentVolumeNotFound
	}

	return parentVolume.NewSubvolume(handle)
}
