package volume

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
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
	GUID       string     `json:"guid"`
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
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrDestroyVolumeFailed = errors.New("failed to destroy volume")

var ErrSetPropertyFailed = errors.New("failed to set property on volume")

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
	CreateVolume(strategy Strategy, properties Properties, ttl *uint) (Volume, error)
	DestroyVolume(handle string) error

	SetProperty(volumeGUID string, propertyName string, propertyValue string) error

	TTL(Volume) time.Duration
}

type repository struct {
	volumeDir  string
	driver     Driver
	defaultTTL TTL

	logger lager.Logger

	setPropertyLock *sync.Mutex
}

func NewRepository(logger lager.Logger, volumeDir string, driver Driver, defaultTTL TTL) Repository {
	return &repository{
		volumeDir:       volumeDir,
		logger:          logger,
		driver:          driver,
		defaultTTL:      defaultTTL,
		setPropertyLock: &sync.Mutex{},
	}
}

func (repo *repository) TTL(volume Volume) time.Duration {
	return volume.TTL.Duration()
}

func (repo *repository) DestroyVolume(handle string) error {
	err := repo.destroyVolume(repo.dataPath(handle))
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

	guid := repo.createUuid()
	newVolumeMetadataPath := repo.metadataPath(guid)
	err := os.Mkdir(newVolumeMetadataPath, 0755)
	if err != nil {
		return repo.handleError(err, "failed-to-create-metadata-dir", ErrCreateVolumeFailed)
	}

	metadata := repo.metadata(guid)
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

	newVolumeDataPath := repo.dataPath(guid)
	err = repo.doStrategy(strategyName, newVolumeDataPath, strategy, logger)
	if err != nil {
		repo.deleteVolumeMetadataDir(guid)
		return Volume{}, err
	}

	return Volume{
		Path:       newVolumeDataPath,
		GUID:       guid,
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) handleError(internalError error, errorMsg string, externalError error) (Volume, error) {
	repo.logger.Error(errorMsg, internalError)
	return Volume{}, externalError
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
		metadata := repo.metadata(volumeDir.Name())
		volumeProperties, err := metadata.Properties()
		if err != nil {
			repo.logger.Error("failed-to-read-properties", err, lager.Data{
				"volume": volumeDir.Name(),
			})
			return nil, err
		}

		ttl, err := metadata.TTL()
		if err != nil {
			repo.logger.Error("failed-to-read-ttl", err, lager.Data{
				"volume": volumeDir.Name(),
			})
			return nil, err
		}

		expiresAt, err := metadata.ExpiresAt()
		if err != nil {
			repo.logger.Error("failed-to-read-expires-at", err, lager.Data{
				"volume": volumeDir.Name(),
			})
			return nil, err
		}

		if volumeProperties.HasProperties(queryProperties) {
			response = append(response, Volume{
				GUID:       volumeDir.Name(),
				Path:       repo.dataPath(volumeDir.Name()),
				Properties: volumeProperties,
				TTL:        ttl,
				ExpiresAt:  expiresAt,
			})
		}
	}

	return response, nil
}

func (repo *repository) SetProperty(volumeGUID string, propertyName string, propertyValue string) error {
	repo.setPropertyLock.Lock()
	defer repo.setPropertyLock.Unlock()

	md := repo.metadata(volumeGUID)

	properties, err := md.Properties()
	if err != nil {
		repo.logger.Error("failed-to-read-properties", err, lager.Data{
			"volume": volumeGUID,
		})
		return err
	}

	properties = properties.UpdateProperty(propertyName, propertyValue)

	err = md.StoreProperties(properties)
	if err != nil {
		_, err = repo.handleError(err, "failed-to-write-properties", ErrSetPropertyFailed)
		return err
	}

	return nil
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
		parentGUID, found := strategy["volume"]
		if !found {
			logger.Error("no-parent-volume-provided", nil)
			return ErrNoParentVolumeProvided
		}

		if !repo.volumeExists(parentGUID) {
			logger.Error("parent-volume-not-found", nil)
			return ErrParentVolumeNotFound
		}

		parentDataPath := repo.dataPath(parentGUID)
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
			"guid": id,
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

func (repo *repository) createUuid() string {
	guid, err := uuid.NewV4()
	if err != nil {
		repo.logger.Fatal("failed-to-generate-guid", err)
	}

	return guid.String()
}

func (repo *repository) volumeExists(guid string) bool {
	info, err := os.Stat(repo.metadataPath(guid))
	if err != nil {
		return false
	}

	return info.IsDir()
}
