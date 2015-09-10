package bomberman

import (
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
)

type bombermanRepository struct {
	volume.Repository
	bomberman *Bomberman
}

func NewBombermanRepository(repo volume.Repository, logger lager.Logger) volume.Repository {
	bombermanRepo := &bombermanRepository{
		Repository: repo,
	}

	setUsUpTheBomb := New(func(handle string) {
		err := bombermanRepo.DestroyVolume(handle)
		if err != nil {
			logger.Error("failed-to-destroy-end-of-life-volume", err, lager.Data{
				"handle": handle,
			})
		}
	})

	bombermanRepo.bomberman = setUsUpTheBomb

	return bombermanRepo
}

func (br *bombermanRepository) CreateVolume(strategy volume.Strategy, properties volume.Properties, ttl *uint) (volume.Volume, error) {
	strategyName, found := strategy["type"]
	if !found {
		return volume.Volume{}, volume.ErrMissingStrategy
	}

	if strategyName == volume.StrategyCopyOnWrite {
		parentHandle, found := strategy["volume"]
		if !found {
			return volume.Volume{}, volume.ErrNoParentVolumeProvided
		}

		br.bomberman.Pause(parentHandle)
	}

	createdVolume, err := br.Repository.CreateVolume(strategy, properties, ttl)
	if err != nil {
		return volume.Volume{}, err
	}

	br.bomberman.Strap(createdVolume)

	return createdVolume, err
}

func (br *bombermanRepository) DestroyVolume(handle string) error {
	parentHandle, found, err := br.Repository.VolumeParent(handle)
	if err != nil {
		return err
	}

	if found {
		defer br.bomberman.Unpause(parentHandle)
	}

	return br.Repository.DestroyVolume(handle)
}

func (br *bombermanRepository) SetProperty(handle string, propertyName string, propertyValue string) error {
	br.bomberman.Pause(handle)
	defer br.bomberman.Unpause(handle)

	return br.Repository.SetProperty(handle, propertyName, propertyValue)
}

func (br *bombermanRepository) SetTTL(handle string, ttl uint) error {
	br.bomberman.Pause(handle)

	err := br.Repository.SetTTL(handle, ttl)
	if err != nil {
		br.bomberman.Unpause(handle)
		return err
	}

	volume, err := br.Repository.GetVolume(handle)
	if err != nil {
		br.bomberman.Unpause(handle)
		return err
	}

	br.bomberman.Defuse(handle)
	br.bomberman.Strap(volume)

	return nil
}
