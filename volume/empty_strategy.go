package volume

type EmptyStrategy struct{}

func (EmptyStrategy) Materialize(handle string, fs Filesystem) (FilesystemInitVolume, error) {
	return fs.NewVolume(handle)
}
