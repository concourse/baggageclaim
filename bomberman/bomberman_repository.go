package bomberman

import (
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
)

type bombermanRepository struct {
	repo      volume.Repository
	bomberman *Bomberman
}

func NewBombermanRepository(repo volume.Repository, logger lager.Logger) volume.Repository {
	setUsUpTheBomb := New(func(handle string) {
		err := repo.DestroyVolume(handle)
		if err != nil {
			logger.Error("failed-to-destroy-end-of-life-volume", err, lager.Data{
				"handle": handle,
			})
		}
	})

	return &bombermanRepository{
		repo:      repo,
		bomberman: setUsUpTheBomb,
	}
}

func (br *bombermanRepository) ListVolumes(queryProperties volume.Properties) (volume.Volumes, error) {
	return br.repo.ListVolumes(queryProperties)
}

func (br *bombermanRepository) GetVolume(handle string) (volume.Volume, error) {
	panic("not implemented")
}

func (br *bombermanRepository) CreateVolume(strategy volume.Strategy, properties volume.Properties, ttl *uint) (volume.Volume, error) {
	createdVolume, err := br.repo.CreateVolume(strategy, properties, ttl)
	if err != nil {
		return volume.Volume{}, err
	}

	br.bomberman.Strap(createdVolume)

	return createdVolume, err
}
func (br *bombermanRepository) DestroyVolume(handle string) error {
	return br.repo.DestroyVolume(handle)
}

func (br *bombermanRepository) SetProperty(handle string, propertyName string, propertyValue string) error {
	br.bomberman.Pause(handle)
	defer br.bomberman.Unpause(handle)

	return br.repo.SetProperty(handle, propertyName, propertyValue)
}

func (br *bombermanRepository) SetTTL(handle string, ttl uint) error {
	br.bomberman.Pause(handle)

	err := br.repo.SetTTL(handle, ttl)
	if err != nil {
		br.bomberman.Unpause(handle)
		return err
	}

	volume, err := br.repo.GetVolume(handle)
	if err != nil {
		br.bomberman.Unpause(handle)
		return err
	}

	br.bomberman.Defuse(handle)
	br.bomberman.Strap(volume)

	return nil
}
