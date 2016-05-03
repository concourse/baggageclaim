package volume

//go:generate counterfeiter . Driver

type Driver interface {
	CreateVolume(path string) error
	DestroyVolume(path string) error
	GetVolumeSize(path string) (uint64, error)

	CreateCopyOnWriteLayer(path string, parent string) error
}
