package reaper

import (
	"fmt"

	"github.com/concourse/baggageclaim/volume"
	"github.com/hashicorp/go-multierror"
	"github.com/pivotal-golang/clock"
)

type Reaper struct {
	clock clock.Clock
	repo  volume.Repository
}

func NewReaper(
	clock clock.Clock,
	repository volume.Repository,
) *Reaper {
	return &Reaper{
		clock: clock,
		repo:  repository,
	}
}

func (reaper *Reaper) Reap() error {
	volumes, err := reaper.repo.ListVolumes(volume.Properties{})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %s", err)
	}

	reapingTime := reaper.clock.Now()

	hasChildren := map[string]bool{}

	for _, volume := range volumes {
		parentVolume, found, err := reaper.repo.VolumeParent(volume.Handle)
		if err != nil {
			return fmt.Errorf("failed to determine volume parent: %s", err)
		}

		if found {
			hasChildren[parentVolume.Handle] = true
		}
	}

	var destroyErrs *multierror.Error

	for _, volume := range volumes {
		if volume.TTL.IsUnlimited() {
			continue
		}

		if hasChildren[volume.Handle] {
			continue
		}

		if reapingTime.After(volume.ExpiresAt) {
			err = reaper.repo.DestroyVolume(volume.Handle)
			if err != nil {
				destroyErrs = multierror.Append(
					destroyErrs,
					fmt.Errorf("failed to destroy %s: %s", volume.Handle, err),
				)

				continue
			}
		}
	}

	return destroyErrs.ErrorOrNil()
}
