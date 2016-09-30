package volume

import (
	"errors"

	"code.cloudfoundry.org/lager"
)

var ErrVolumeDoesNotExist = errors.New("volume does not exist")

//go:generate counterfeiter . Repository

type Repository interface {
	ListVolumes(queryProperties Properties) (Volumes, error)
	GetVolume(handle string) (Volume, bool, error)
	GetVolumeStats(handle string) (VolumeStats, bool, error)
	CreateVolume(handle string, strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error)
	DestroyVolume(handle string) error

	SetProperty(handle string, propertyName string, propertyValue string) error
	SetTTL(handle string, ttl uint) error

	VolumeParent(handle string) (Volume, bool, error)
}

type repository struct {
	logger lager.Logger

	filesystem Filesystem

	locker LockManager
}

func NewRepository(
	logger lager.Logger,
	filesystem Filesystem,
	locker LockManager,
) Repository {
	return &repository{
		logger:     logger,
		filesystem: filesystem,
		locker:     locker,
	}
}

func (repo *repository) DestroyVolume(handle string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("destroy-volume", lager.Data{
		"volume": handle,
	})

	volume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return err
	}

	if !found {
		logger.Info("volume-not-found")
		return ErrVolumeDoesNotExist
	}

	err = volume.Destroy()
	if err != nil {
		logger.Error("failed-to-destroy", err)
		return err
	}

	return nil
}

func (repo *repository) CreateVolume(handle string, strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error) {
	logger := repo.logger.Session("create-volume", lager.Data{"handle": handle})

	initVolume, err := strategy.Materialize(logger, handle, repo.filesystem)
	if err != nil {
		logger.Error("failed-to-materialize-strategy", err)
		return Volume{}, err
	}

	var initialized bool
	defer func() {
		if !initialized {
			initVolume.Destroy()
		}
	}()

	err = initVolume.StoreProperties(properties)
	if err != nil {
		logger.Error("failed-to-set-properties", err)
		return Volume{}, err
	}

	ttl := TTL(ttlInSeconds)

	expiresAt, err := initVolume.StoreTTL(ttl)
	if err != nil {
		logger.Error("failed-to-set-properties", err)
		return Volume{}, err
	}

	liveVolume, err := initVolume.Initialize()
	if err != nil {
		logger.Error("failed-to-initialize-volume", err)
		return Volume{}, err
	}

	initialized = true

	return Volume{
		Handle:     liveVolume.Handle(),
		Path:       liveVolume.DataPath(),
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) ListVolumes(queryProperties Properties) (Volumes, error) {
	logger := repo.logger.Session("list-volumes", lager.Data{})

	liveVolumes, err := repo.filesystem.ListVolumes()
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		return Volumes{}, err
	}

	response := make(Volumes, 0, len(liveVolumes))

	for _, liveVolume := range liveVolumes {
		volume, err := repo.volumeFrom(liveVolume)
		if err == ErrVolumeDoesNotExist {
			continue
		}

		if err != nil {
			logger.Error("failed-hydrating-volume", err)
			return nil, err
		}

		if volume.Properties.HasProperties(queryProperties) {
			response = append(response, volume)
		}
	}

	return response, nil
}

func (repo *repository) GetVolume(handle string) (Volume, bool, error) {
	logger := repo.logger.Session("get-volume", lager.Data{
		"volume": handle,
	})

	liveVolume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return Volume{}, false, err
	}

	if !found {
		logger.Info("volume-not-found")
		return Volume{}, false, nil
	}

	volume, err := repo.volumeFrom(liveVolume)
	if err == ErrVolumeDoesNotExist {
		return Volume{}, false, nil
	}

	if err != nil {
		logger.Error("failed-to-hydrate-volume", err)
		return Volume{}, false, err
	}

	return volume, true, nil
}

func (repo *repository) GetVolumeStats(handle string) (VolumeStats, bool, error) {
	logger := repo.logger.Session("get-volume", lager.Data{
		"volume": handle,
	})

	liveVolume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return VolumeStats{}, false, err
	}

	if !found {
		logger.Info("volume-not-found")
		return VolumeStats{}, false, nil
	}

	size, err := liveVolume.SizeInBytes()
	if err != nil {
		logger.Error("failed-to-get-volume-stats", err)
		return VolumeStats{}, false, err
	}

	stats := VolumeStats{
		SizeInBytes: size,
	}

	return stats, true, nil
}

func (repo *repository) SetProperty(handle string, propertyName string, propertyValue string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("set-property")

	volume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return err
	}

	if !found {
		logger.Info("volume-not-found")
		return ErrVolumeDoesNotExist
	}

	properties, err := volume.LoadProperties()
	if err != nil {
		logger.Error("failed-to-read-properties", err, lager.Data{
			"volume": handle,
		})
		return err
	}

	properties = properties.UpdateProperty(propertyName, propertyValue)

	err = volume.StoreProperties(properties)
	if err != nil {
		logger.Error("failed-to-store-properties", err)
		return err
	}

	return nil
}

func (repo *repository) SetTTL(handle string, ttl uint) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("set-ttl")

	volume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return err
	}

	if !found {
		logger.Info("volume-not-found")
		return ErrVolumeDoesNotExist
	}

	_, err = volume.StoreTTL(TTL(ttl))
	if err != nil {
		logger.Error("failed-to-store-ttl", err)
		return err
	}

	return nil
}

func (repo *repository) VolumeParent(handle string) (Volume, bool, error) {
	logger := repo.logger.Session("volume-parent")

	liveVolume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return Volume{}, false, err
	}

	if !found {
		logger.Info("volume-not-found")
		return Volume{}, false, ErrVolumeDoesNotExist
	}

	parentVolume, found, err := liveVolume.Parent()
	if err != nil {
		logger.Error("failed-to-get-parent-volume", err)
		return Volume{}, false, err
	}

	if !found {
		return Volume{}, false, nil
	}

	volume, err := repo.volumeFrom(parentVolume)
	if err != nil {
		logger.Error("failed-to-hydrate-parent-volume", err)
		return Volume{}, false, err
	}

	return volume, true, nil
}

func (repo *repository) volumeFrom(liveVolume FilesystemLiveVolume) (Volume, error) {
	properties, err := liveVolume.LoadProperties()
	if err != nil {
		return Volume{}, err
	}

	ttl, expiresAt, err := liveVolume.LoadTTL()
	if err != nil {
		return Volume{}, err
	}

	return Volume{
		Handle:     liveVolume.Handle(),
		Path:       liveVolume.DataPath(),
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}
