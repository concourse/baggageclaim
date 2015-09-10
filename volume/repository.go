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

	VolumeParent(handle string) (string, bool, error)
}

type repository struct {
	volumeDir  string
	limboDir   string
	driver     Driver
	locker     LockManager
	defaultTTL TTL

	logger lager.Logger
}

const (
	liveDir = "live"
	deadDir = "dead"
)

func NewRepository(
	logger lager.Logger,
	driver Driver,
	locker LockManager,
	parentDir string,
	defaultTTL TTL,
) (Repository, error) {
	volumeDir := filepath.Join(parentDir, liveDir)
	err := os.MkdirAll(volumeDir, 0755)
	if err != nil {
		return nil, err
	}

	limboDir := filepath.Join(parentDir, deadDir)
	err = os.MkdirAll(limboDir, 0755)
	if err != nil {
		return nil, err
	}

	return &repository{
		volumeDir:  volumeDir,
		limboDir:   limboDir,
		logger:     logger,
		driver:     driver,
		locker:     locker,
		defaultTTL: defaultTTL,
	}, nil
}

func (repo *repository) DestroyVolume(handle string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("destroy-volume", lager.Data{
		"volume": handle,
	})

	err := os.Rename(repo.metadataPath(handle), repo.deadPath(handle))
	if err != nil {
		logger.Error("failed-to-move-to-dead", err)
		return ErrDestroyVolumeFailed
	}

	err = repo.destroyVolume(repo.deadPath(handle))
	if err != nil {
		logger.Error("failed-to-delete-data", err)
		return ErrDestroyVolumeFailed
	}

	err = os.RemoveAll(repo.deadPath(handle))
	if err != nil {
		logger.Error("failed-to-delete-metadata", err)
		return ErrDestroyVolumeFailed
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
		logger.Error("failed-to-create-metadata-dir", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	metadata := repo.metadata(handle)
	err = metadata.StoreProperties(properties)
	if err != nil {
		logger.Error("failed-to-create-properties-file", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	err = metadata.StoreTTL(ttl)
	if err != nil {
		logger.Error("failed-to-create-ttl-file", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	expiresAt, err := metadata.ExpiresAt()
	if err != nil {
		logger.Error("failed-to-read-expires-at", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	err = metadata.StoreState(CreatingVolume)
	if err != nil {
		logger.Error("failed-to-create-state-file", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	newVolumeDataPath := repo.dataPath(handle)
	err = repo.doStrategy(strategyName, handle, newVolumeDataPath, strategy, logger)
	if err != nil {
		repo.deleteVolumeMetadataDir(handle)
		return Volume{}, err
	}

	err = metadata.StoreState(VolumeActive)
	if err != nil {
		logger.Error("failed-to-create-state-file", err)
		return Volume{}, ErrCreateVolumeFailed
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
	logger := repo.logger.Session("list-volumes", lager.Data{})

	volumeDirs, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumeDirs))

	for _, volumeDir := range volumeDirs {
		vol, err := repo.GetVolume(volumeDir.Name())
		if err != nil {
			logger.Error("failed-to-get-volume", err, lager.Data{
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
	logger := repo.logger.Session("get-volume", lager.Data{
		"volume": handle,
	})

	if !repo.volumeExists(handle) {
		logger.Error("failed-to-get-volume", errors.New("volume-does-not-exist"))
		return Volume{}, ErrGetVolumeFailed
	}

	metadata := repo.metadata(handle)
	volumeProperties, err := metadata.Properties()
	if err != nil {
		logger.Error("failed-to-read-properties", err)
		return Volume{}, ErrGetVolumeFailed
	}

	ttl, err := metadata.TTL()
	if err != nil {
		logger.Error("failed-to-read-ttl", err)
		return Volume{}, ErrGetVolumeFailed
	}

	expiresAt, err := metadata.ExpiresAt()
	if err != nil {
		logger.Error("failed-to-read-expires-at", err)
		return Volume{}, ErrGetVolumeFailed
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

	logger := repo.logger.Session("set-property")

	md := repo.metadata(handle)

	properties, err := md.Properties()
	if err != nil {
		logger.Error("failed-to-read-properties", err, lager.Data{
			"volume": handle,
		})
		return ErrSetPropertyFailed
	}

	properties = properties.UpdateProperty(propertyName, propertyValue)

	err = md.StoreProperties(properties)
	if err != nil {
		logger.Error("failed-to-store-properties", err)
		return ErrSetPropertyFailed
	}

	return nil
}

func (repo *repository) SetTTL(handle string, ttl uint) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("set-ttl")

	err := repo.metadata(handle).StoreTTL(TTL(ttl))
	if err != nil {
		logger.Error("failed-to-store-ttl", err)
		return ErrSetTTLFailed
	}

	return nil
}

func (repo *repository) VolumeParent(handle string) (string, bool, error) {
	parentDir, err := filepath.EvalSymlinks(filepath.Join(repo.metadataPath(handle), "parent"))
	if os.IsNotExist(err) {
		return "", false, nil
	}

	if err != nil {
		return "", false, err
	}

	return filepath.Base(parentDir), true, nil
}

func (repo *repository) doStrategy(strategyName string, newVolumeHandle string, newVolumeDataPath string, strategy Strategy, logger lager.Logger) error {
	logger = repo.logger.Session("do-strategy")

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

		err = os.Symlink(repo.metadataPath(parentHandle), filepath.Join(repo.metadataPath(newVolumeHandle), "parent"))
		if err != nil {
			logger.Error("failed-to-symlink-to-parent", err)
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

func (repo *repository) deadPath(id string) string {
	return filepath.Join(repo.limboDir, id)
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
			"volume": id,
		})
	}
}

func (repo *repository) destroyVolume(limboPath string) error {
	return repo.driver.DestroyVolume(filepath.Join(limboPath, "volume"))
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
