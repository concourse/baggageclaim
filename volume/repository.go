package volume

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/lager"
)

type Strategy map[string]string

const (
	StrategyEmpty       = "empty"
	StrategyCopyOnWrite = "cow"
)

type Volume struct {
	Handle     string     `json:"handle"`
	Path       string     `json:"path"`
	Properties Properties `json:"properties"`
	TTL        TTL        `json:"ttl,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

type TTLProperties struct {
}

type Volumes []Volume

var ErrMissingStrategy = errors.New("missing strategy")
var ErrUnrecognizedStrategy = errors.New("unrecognized strategy")

var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrGetVolumeFailed = errors.New("failed to get volume")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrDestroyVolumeFailed = errors.New("failed to destroy volume")

var ErrSetPropertyFailed = errors.New("failed to set property on volume")
var ErrSetTTLFailed = errors.New("failed to set ttl on volume")

var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

//go:generate counterfeiter . Driver

type Driver interface {
	CreateVolume(path string) error
	DestroyVolume(path string) error

	CreateCopyOnWriteLayer(path string, parent string) error
}

//go:generate counterfeiter . Repository

type Repository interface {
	ListVolumes(queryProperties Properties) (Volumes, error)
	GetVolume(handle string) (Volume, error)
	CreateVolume(strategy Strategy, properties Properties, ttl *uint) (Volume, error)
	DestroyVolume(handle string) error

	SetProperty(handle string, propertyName string, propertyValue string) error
	SetTTL(handle string, ttl uint) error
}

type repository struct {
	volumeDir  string
	driver     Driver
	locker     LockManager
	defaultTTL TTL

	logger lager.Logger
}

func NewRepository(
	logger lager.Logger,
	driver Driver,
	locker LockManager,
	volumeDir string,
	defaultTTL TTL,
) Repository {
	return &repository{
		volumeDir:  volumeDir,
		logger:     logger,
		driver:     driver,
		locker:     locker,
		defaultTTL: defaultTTL,
	}
}

func (repo *repository) DestroyVolume(handle string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	err := repo.metadata(handle).StoreState(VolumeDestroyed)

	if err != nil {
		_, err = repo.handleError(err, "failed-to-set-volume-state-to-destroy", ErrDestroyVolumeFailed)
		return err
	}

	err = repo.destroyVolume(repo.dataPath(handle))
	if err != nil {
		_, err = repo.handleError(err, "failed-to-delete-data", ErrDestroyVolumeFailed)
		return err
	}

	err = os.RemoveAll(repo.metadataPath(handle))
	if err != nil {
		_, err = repo.handleError(err, "failed-to-delete-metadata", ErrDestroyVolumeFailed)
		return err
	}

	return nil
}

type TTL uint

func (ttl TTL) Duration() time.Duration {
	return time.Duration(ttl) * time.Second
}

func (repo *repository) CreateVolume(strategy Strategy, properties Properties, ttlInt *uint) (Volume, error) {
	strategyName, found := strategy["type"]
	if !found {
		return Volume{}, ErrMissingStrategy
	}

	logger := repo.logger.Session("create-volume", lager.Data{
		"strategy": strategyName,
	})

	ttl := TTL(repo.defaultTTL)
	if ttlInt != nil {
		ttl = TTL(*ttlInt)
	}

	handle := repo.generateHandle()
	newVolumeMetadataPath := repo.metadataPath(handle)
	err := os.Mkdir(newVolumeMetadataPath, 0755)
	if err != nil {
		return repo.handleError(err, "failed-to-create-metadata-dir", ErrCreateVolumeFailed)
	}

	metadata := repo.metadata(handle)
	err = metadata.StoreProperties(properties)
	if err != nil {
		return repo.handleError(err, "failed-to-create-properties-file", ErrCreateVolumeFailed)
	}

	err = metadata.StoreTTL(ttl)
	if err != nil {
		return repo.handleError(err, "failed-to-create-ttl-file", ErrCreateVolumeFailed)
	}

	expiresAt, err := metadata.ExpiresAt()
	if err != nil {
		return repo.handleError(err, "failed-to-read-expires-at", ErrCreateVolumeFailed)
	}

	err = metadata.StoreState(CreatingVolume)
	if err != nil {
		return repo.handleError(err, "failed-to-create-state-file", ErrCreateVolumeFailed)
	}

	newVolumeDataPath := repo.dataPath(handle)
	err = repo.doStrategy(strategyName, newVolumeDataPath, strategy, logger)
	if err != nil {
		repo.deleteVolumeMetadataDir(handle)
		return Volume{}, err
	}

	err = metadata.StoreState(VolumeActive)
	if err != nil {
		return repo.handleError(err, "failed-to-create-state-file", ErrCreateVolumeFailed)
	}

	return Volume{
		Path:       newVolumeDataPath,
		Handle:     handle,
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) ListVolumes(queryProperties Properties) (Volumes, error) {
	volumeDirs, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		repo.logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumeDirs))

	for _, volumeDir := range volumeDirs {
		vol, err := repo.GetVolume(volumeDir.Name())
		if err != nil {
			repo.logger.Error("failed-to-get-volume", err, lager.Data{
				"volume": volumeDir.Name(),
			})
			return nil, err
		}

		if vol.Handle == "" {
			continue
		}

		if vol.Properties.HasProperties(queryProperties) {
			response = append(response, vol)
		}
	}

	return response, nil
}

func (repo *repository) GetVolume(handle string) (Volume, error) {
	if !repo.volumeActive(handle) {
		return Volume{}, nil
	}

	if !repo.volumeExists(handle) {
		return repo.handleError(errors.New("volume-does-not-exist"), "failed-to-get-volume", ErrGetVolumeFailed)
	}

	metadata := repo.metadata(handle)
	volumeProperties, err := metadata.Properties()
	if err != nil {
		return repo.handleError(err, "failed-to-read-properties", ErrGetVolumeFailed)
	}

	ttl, err := metadata.TTL()
	if err != nil {
		return repo.handleError(err, "failed-to-read-ttl", ErrGetVolumeFailed)
	}

	expiresAt, err := metadata.ExpiresAt()
	if err != nil {
		return repo.handleError(err, "failed-to-read-expires-at", ErrGetVolumeFailed)
	}

	return Volume{
		Handle:     handle,
		Path:       repo.dataPath(handle),
		Properties: volumeProperties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) SetProperty(handle string, propertyName string, propertyValue string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	md := repo.metadata(handle)

	properties, err := md.Properties()
	if err != nil {
		repo.logger.Error("failed-to-read-properties", err, lager.Data{
			"volume": handle,
		})
		return err
	}

	properties = properties.UpdateProperty(propertyName, propertyValue)

	err = md.StoreProperties(properties)
	if err != nil {
		_, err = repo.handleError(err, "failed-to-store-properties", ErrSetPropertyFailed)
		return err
	}

	return nil
}

func (repo *repository) SetTTL(handle string, ttl uint) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	err := repo.metadata(handle).StoreTTL(TTL(ttl))
	if err != nil {
		_, err = repo.handleError(err, "failed-to-store-ttl", ErrSetTTLFailed)
		return err
	}

	return nil
}

func (repo *repository) handleError(internalError error, errorMsg string, externalError error) (Volume, error) {
	repo.logger.Error(errorMsg, internalError)
	return Volume{}, externalError
}

func (repo *repository) doStrategy(strategyName string, newVolumeDataPath string, strategy Strategy, logger lager.Logger) error {
	switch strategyName {
	case StrategyEmpty:
		err := repo.createEmptyVolume(newVolumeDataPath)
		if err != nil {
			logger.Error("failed-to-create-volume", err, lager.Data{
				"path": newVolumeDataPath,
			})
			return ErrCreateVolumeFailed
		}

	case StrategyCopyOnWrite:
		parentHandle, found := strategy["volume"]
		if !found {
			logger.Error("no-parent-volume-provided", nil)
			return ErrNoParentVolumeProvided
		}

		repo.locker.Lock(parentHandle)
		defer repo.locker.Unlock(parentHandle)

		if !repo.volumeExists(parentHandle) {
			logger.Error("parent-volume-not-found", nil)
			return ErrParentVolumeNotFound
		}

		parentDataPath := repo.dataPath(parentHandle)
		err := repo.createCowVolume(parentDataPath, newVolumeDataPath)
		if err != nil {
			logger.Error("failed-to-copy-volume", err)
			return ErrCreateVolumeFailed
		}

	default:
		logger.Error("unrecognized-strategy", nil, lager.Data{
			"strategy": strategyName,
		})
		return ErrUnrecognizedStrategy
	}

	return nil
}

func (repo *repository) metadataPath(id string) string {
	return filepath.Join(repo.volumeDir, id)
}

func (repo *repository) metadata(id string) *Metadata {
	return &Metadata{path: repo.metadataPath(id)}
}

func (repo *repository) dataPath(id string) string {
	return filepath.Join(repo.metadataPath(id), "volume")
}

func (repo *repository) deleteVolumeMetadataDir(id string) {
	err := os.RemoveAll(repo.metadataPath(id))
	if err != nil {
		repo.logger.Error("failed-to-cleanup", err, lager.Data{
			"handle": id,
		})
	}
}

func (repo *repository) destroyVolume(volumePath string) error {
	return repo.driver.DestroyVolume(volumePath)
}

func (repo *repository) createEmptyVolume(volumePath string) error {
	return repo.driver.CreateVolume(volumePath)
}

func (repo *repository) createCowVolume(parentPath string, newPath string) error {
	return repo.driver.CreateCopyOnWriteLayer(newPath, parentPath)
}

func (repo *repository) generateHandle() string {
	handle, err := uuid.NewV4()
	if err != nil {
		repo.logger.Fatal("failed-to-generate-handle", err)
	}

	return handle.String()
}

func (repo *repository) volumeActive(handle string) bool {
	volumeState, err := repo.metadata(handle).VolumeState()

	if err != nil {
		return false
	}

	return volumeState == VolumeActive
}

func (repo *repository) volumeExists(handle string) bool {
	info, err := os.Stat(repo.metadataPath(handle))
	if err != nil {
		return false
	}

	return info.IsDir()
}
