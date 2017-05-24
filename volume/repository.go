package volume

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/baggageclaim/uidgid"

	"code.cloudfoundry.org/lager"
)

var ErrVolumeDoesNotExist = errors.New("volume does not exist")
var ErrVolumeIsCorrupted = errors.New("volume is corrupted")

//go:generate counterfeiter . Repository

type Repository interface {
	ListVolumes(queryProperties Properties) (Volumes, []string, error)
	GetVolume(handle string) (Volume, bool, error)
	GetVolumeStats(handle string) (VolumeStats, bool, error)
	CreateVolume(handle string, strategy Strategy, properties Properties, ttlInSeconds uint, isPrivileged bool) (Volume, error)
	DestroyVolume(handle string) error
	DestroyVolumeAndDescendants(handle string) error

	SetProperty(handle string, propertyName string, propertyValue string) error
	SetTTL(handle string, ttl uint) error
	SetPrivileged(handle string, privileged bool) error

	StreamIn(handle string, path string, stream io.Reader) (bool, error)
	StreamOut(handle string, path string, dest io.Writer) error

	VolumeParent(handle string) (Volume, bool, error)
}

type repository struct {
	logger lager.Logger

	filesystem Filesystem

	locker LockManager

	namespacer func(bool) uidgid.Namespacer
}

func NewRepository(
	logger lager.Logger,
	filesystem Filesystem,
	locker LockManager,
	privilegedNamespacer uidgid.Namespacer,
	unprivilegedNamespacer uidgid.Namespacer,
) Repository {
	return &repository{
		logger:     logger,
		filesystem: filesystem,
		locker:     locker,

		namespacer: func(privileged bool) uidgid.Namespacer {
			if privileged {
				return privilegedNamespacer
			} else {
				return unprivilegedNamespacer
			}
		},
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

func (repo *repository) CreateVolume(handle string, strategy Strategy, properties Properties, ttlInSeconds uint, isPrivileged bool) (Volume, error) {
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

	err = initVolume.StorePrivileged(isPrivileged)
	if err != nil {
		logger.Error("failed-to-set-privileged", err)
		return Volume{}, err
	}

	err = repo.namespacer(isPrivileged).NamespacePath(logger, initVolume.DataPath())
	if err != nil {
		logger.Error("failed-to-namespace-data", err)
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
	logger := repo.logger.Session("list-volumes")

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
	logger := repo.logger.Session("get-volume-stats", lager.Data{
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

	logger := repo.logger.Session("set-property", lager.Data{
		"volume":   handle,
		"property": propertyName,
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

	logger := repo.logger.Session("set-ttl", lager.Data{
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

	_, err = volume.StoreTTL(TTL(ttl))
	if err != nil {
		logger.Error("failed-to-store-ttl", err)
		return err
	}

	return nil
}

func (repo *repository) SetPrivileged(handle string, privileged bool) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("set-privileged", lager.Data{
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

	err = repo.namespacer(privileged).NamespacePath(logger, volume.DataPath())
	if err != nil {
		logger.Error("failed-to-namespace-volume", err)
		return err
	}

	err = volume.StorePrivileged(privileged)
	if err != nil {
		logger.Error("failed-to-store-privileged", err)
		return err
	}

	return nil
}

func (repo *repository) StreamIn(handle string, path string, stream io.Reader) (bool, error) {
	logger := repo.logger.Session("stream-in", lager.Data{
		"volume":   handle,
		"sub-path": path,
	})

	volume, found, err := repo.filesystem.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-lookup-volume", err)
		return false, err
	}

	if !found {
		logger.Info("volume-not-found")
		return false, ErrVolumeDoesNotExist
	}

	destinationPath := filepath.Join(volume.DataPath(), path)

	logger = logger.WithData(lager.Data{
		"full-path": destinationPath,
	})

	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		logger.Error("failed-to-create-destination-path", err)
		return false, err
	}

	privileged, err := volume.LoadPrivileged()
	if err != nil {
		logger.Error("failed-to-check-if-volume-is-privileged", err)
		return false, err
	}

	err = repo.namespacer(privileged).NamespacePath(logger, volume.DataPath())
	if err != nil {
		logger.Error("failed-to-namespace-path", err)
		return false, err
	}

	return repo.streamIn(stream, destinationPath, privileged)
}

func (repo *repository) StreamOut(handle string, path string, dest io.Writer) error {
	logger := repo.logger.Session("stream-in", lager.Data{
		"volume":   handle,
		"sub-path": path,
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

	srcPath := filepath.Join(volume.DataPath(), path)

	logger = logger.WithData(lager.Data{
		"full-path": srcPath,
	})

	isPrivileged, err := volume.LoadPrivileged()
	if err != nil {
		logger.Error("failed-to-check-if-volume-is-privileged", err)
		return err
	}

	return repo.streamOut(dest, srcPath, isPrivileged)
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

	isPrivileged, err := liveVolume.LoadPrivileged()
	if err != nil {
		return Volume{}, err
	}

	return Volume{
		Handle:     liveVolume.Handle(),
		Path:       liveVolume.DataPath(),
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
		Privileged: isPrivileged,
	}, nil
}
