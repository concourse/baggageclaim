package volume

//go:generate counterfeiter . Driver

type Driver interface {
	CreateVolume(path string) error
	DestroyVolume(path string) error
	GetVolumeSize(path string) (uint, error)

	CreateCopyOnWriteLayer(path string, parent string) error
}
