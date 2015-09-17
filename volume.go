package baggageclaim

import (
	"time"

	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/clock"
)

type Volume interface {
	Handle() string
	Path() string
	TTL() uint
	ExpiresAt() time.Time
	Properties() volume.Properties

	Heartbeat(time.Duration, clock.Clock)
	Release()
}

type Volumes []Volume

type VolumeProperties map[string]string
