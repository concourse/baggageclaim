package volume

//go:generate counterfeiter . Strategy

type Strategy interface {
	Materialize(string, Filesystem) (FilesystemInitVolume, error)
}
