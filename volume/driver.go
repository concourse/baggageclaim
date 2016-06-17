package volume

//go:generate counterfeiter . Driver

type Driver interface {
	CreateVolume(path string) error
	DestroyVolume(path string) error
	GetVolumeSizeInBytes(path string) (int64, error)

	CreateCopyOnWriteLayer(path string, parent string) error
}
