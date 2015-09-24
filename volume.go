package baggageclaim

import (
	"time"

	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Volume

type Volume interface {
	Handle() string
	Path() string
	TTL() uint
	ExpiresAt() time.Time
	Properties() volume.Properties

	Heartbeat(lager.Logger, time.Duration, clock.Clock)
	Release()
}

type Volumes []Volume

type VolumeProperties map[string]string
