package volume

import "github.com/pivotal-golang/lager"

type EmptyStrategy struct{}

func (EmptyStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	return fs.NewVolume(handle)
}
