package volume

import "code.cloudfoundry.org/lager"

type EmptyStrategy struct{}

func (EmptyStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	return fs.NewVolume(handle)
}
