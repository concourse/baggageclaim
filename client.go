package baggageclaim

import "github.com/concourse/baggageclaim/volume"

//go:generate counterfeiter . Client

type Client interface {
	CreateCOWVolume(volumeSpec VolumeSpec) (Volume, error)
	CreateEmptyVolume(volumeSpec VolumeSpec) (Volume, error)

	FindVolumes(properties VolumeProperties) (Volumes, error)
	GetVolumes() (Volumes, error)
	GetVolume(handle string) (Volume, error)

	SetTTL(handle string, ttl uint) error
	SetProperty(handle string, propertyName string, propertyValue string) error
}

type VolumeSpec struct {
	Properties   volume.Properties
	ParentHandle string
	TTLInSeconds uint
}
