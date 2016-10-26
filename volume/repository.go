package volume

import (
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/nu7hatch/gouuid"
)

var ErrVolumeDoesNotExist = errors.New("volume does not exist")
var ErrVolumeIsCorrupted = errors.New("volume is corrupted")

//go:generate counterfeiter . Repository

type Repository interface {
	ListVolumes(queryProperties Properties) (Volumes, []string, error)
	GetVolume(handle string) (Volume, bool, error)
	GetVolumeStats(handle string) (VolumeStats, bool, error)
	CreateVolume(strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error)
	DestroyVolume(handle string) error
	DestroyVolumeAndDescendants(handle string) error

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

	logger.Info("destroyed")

	return nil
}

func (repo *repository) DestroyVolumeAndDescendants(handle string) error {
	allVolumes, err := repo.filesystem.ListVolumes()
	if err != nil {
		return err
	}

	found := false
	for _, candidate := range allVolumes {
		if candidate.Handle() == handle {
			found = true
		}
	}
	if !found {
		return ErrVolumeDoesNotExist
	}

	for _, candidate := range allVolumes {
		candidateParent, found, err := candidate.Parent()
		if err != nil {
			continue
		}
		if !found {
			continue
		}

		if candidateParent.Handle() == handle {
			err = repo.DestroyVolumeAndDescendants(candidate.Handle())
			if err != nil {
				return err
			}
		}
	}

	return repo.DestroyVolume(handle)
}

func (repo *repository) CreateVolume(strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error) {
	handle := repo.generateHandle()

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

func (repo *repository) ListVolumes(queryProperties Properties) (Volumes, []string, error) {
	logger := repo.logger.Session("list-volumes", lager.Data{})

	liveVolumes, err := repo.filesystem.ListVolumes()
	if err != nil {
		logger.Error("failed-to-list-volumes", err)
		return nil, nil, err
	}

	healthyVolumes := make(Volumes, 0, len(liveVolumes))
	corruptedVolumeHandles := []string{}

	for _, liveVolume := range liveVolumes {
		volume, err := repo.volumeFrom(liveVolume)
		if err == ErrVolumeDoesNotExist {
			continue
		}

		if err != nil {
			corruptedVolumeHandles = append(corruptedVolumeHandles, liveVolume.Handle())
			logger.Error("failed-hydrating-volume", err)
			continue
		}

		if volume.Properties.HasProperties(queryProperties) {
			healthyVolumes = append(healthyVolumes, volume)
		}
	}

	return healthyVolumes, corruptedVolumeHandles, nil
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
		return Volume{}, true, ErrVolumeIsCorrupted
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

func (repo *repository) generateHandle() string {
	handle, err := uuid.NewV4()
	if err != nil {
		repo.logger.Fatal("failed-to-generate-handle", err)
	}

	return handle.String()
}
