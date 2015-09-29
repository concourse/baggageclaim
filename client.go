package baggageclaim

import (
	"encoding/json"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Client

type Client interface {
	CreateVolume(lager.Logger, VolumeSpec) (Volume, error)
	ListVolumes(lager.Logger, VolumeProperties) (Volumes, error)
	LookupVolume(lager.Logger, string) (Volume, error)
}

//go:generate counterfeiter . Volume

type Volume interface {
	Handle() string
	Path() string

	SetTTL(timeInSeconds uint) error
	SetProperty(string, string) error

	Expiration() (uint, time.Time, error)
	Properties() (VolumeProperties, error)

	Release()
}

type Volumes []Volume

type VolumeProperties map[string]string

type VolumeSpec struct {
	Strategy     Strategy
	Properties   VolumeProperties
	TTLInSeconds uint
	Privileged   bool
}

type Strategy interface {
	Encode() *json.RawMessage
}

type COWStrategy struct {
	Parent Volume
}

func (strategy COWStrategy) Encode() *json.RawMessage {
	payload, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Volume string `json:"volume"`
	}{
		Type:   "cow",
		Volume: strategy.Parent.Handle(),
	})

	msg := json.RawMessage(payload)
	return &msg
}

type EmptyStrategy struct{}

func (EmptyStrategy) Encode() *json.RawMessage {
	msg := json.RawMessage(`{"type":"empty"}`)
	return &msg
}
