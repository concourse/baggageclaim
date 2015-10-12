package volume

import "errors"

var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

type CowStrategy struct {
	ParentHandle string
}

func (strategy CowStrategy) Materialize(handle string, fs Filesystem) (FilesystemInitVolume, error) {
	if strategy.ParentHandle == "" {
		return nil, ErrNoParentVolumeProvided
	}

	parentVolume, found, err := fs.LookupVolume(strategy.ParentHandle)
	if err != nil {
		return nil, err
	}

	if !found {
		return nil, ErrParentVolumeNotFound
	}

	return parentVolume.NewSubvolume(handle)
}
