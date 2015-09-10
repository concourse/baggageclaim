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
	CreateVolume(strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error)
	DestroyVolume(handle string) error

	SetProperty(handle string, propertyName string, propertyValue string) error
	SetTTL(handle string, ttl uint) error

	VolumeParent(handle string) (Volume, bool, error)
}

type repository struct {
	initDir string
	liveDir string
	deadDir string

	driver     Driver
	locker     LockManager
	defaultTTL TTL

	logger lager.Logger
}

const (
	initDirname = "init" // volumes being initialized
	liveDirname = "live" // volumes accessible via API
	deadDirname = "dead" // volumes being torn down
)

func NewRepository(
	logger lager.Logger,
	driver Driver,
	locker LockManager,
	parentDir string,
) (Repository, error) {
	initDir := filepath.Join(parentDir, initDirname)
	err := os.MkdirAll(initDir, 0755)
	if err != nil {
		return nil, err
	}

	liveDir := filepath.Join(parentDir, liveDirname)
	err = os.MkdirAll(liveDir, 0755)
	if err != nil {
		return nil, err
	}

	deadDir := filepath.Join(parentDir, deadDirname)
	err = os.MkdirAll(deadDir, 0755)
	if err != nil {
		return nil, err
	}

	return &repository{
		initDir: initDir,
		liveDir: liveDir,
		deadDir: deadDir,
		logger:  logger,
		driver:  driver,
		locker:  locker,
	}, nil
}

func (repo *repository) DestroyVolume(handle string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("destroy-volume", lager.Data{
		"volume": handle,
	})

	err := os.Rename(repo.liveMetadataPath(handle), repo.deadMetadataPath(handle))
	if err != nil {
		logger.Error("failed-to-move-to-dead", err)
		return ErrDestroyVolumeFailed
	}

	err = repo.driver.DestroyVolume(repo.deadDataPath(handle))
	if err != nil {
		logger.Error("failed-to-delete-data", err)
		return ErrDestroyVolumeFailed
	}

	err = os.RemoveAll(repo.deadMetadataPath(handle))
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

func (ttl TTL) IsUnlimited() bool {
	return ttl == 0
}

func (repo *repository) CreateVolume(strategy Strategy, properties Properties, ttlInSeconds uint) (Volume, error) {
	strategyName, found := strategy["type"]
	if !found {
		return Volume{}, ErrMissingStrategy
	}

	logger := repo.logger.Session("create-volume", lager.Data{
		"strategy": strategyName,
	})

	ttl := TTL(ttlInSeconds)

	handle := repo.generateHandle()

	newVolumeMetadataPath := repo.initMetadataPath(handle)
	err := os.Mkdir(newVolumeMetadataPath, 0755)
	if err != nil {
		logger.Error("failed-to-create-metadata-dir", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	metadata := repo.initMetadata(handle)

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

	newVolumeDataPath := repo.initDataPath(handle)

	err = repo.doStrategy(strategyName, newVolumeMetadataPath, newVolumeDataPath, strategy, logger)
	if err != nil {
		repo.cleanupFailedInitVolumeMetadataDir(handle)
		return Volume{}, err
	}

	err = os.Rename(repo.initMetadataPath(handle), repo.liveMetadataPath(handle))
	if err != nil {
		repo.cleanupFailedInitVolumeMetadataDir(handle)
		logger.Error("failed-to-move-from-init-to-live", err)
		return Volume{}, err
	}

	return Volume{
		Path:       repo.liveDataPath(handle),
		Handle:     handle,
		Properties: properties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) ListVolumes(queryProperties Properties) (Volumes, error) {
	logger := repo.logger.Session("list-volumes", lager.Data{})

	liveDirs, err := ioutil.ReadDir(repo.liveDir)
	if err != nil {
		logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.liveDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(liveDirs))

	for _, liveDir := range liveDirs {
		vol, err := repo.GetVolume(liveDir.Name())
		if err != nil {
			logger.Error("failed-to-get-volume", err, lager.Data{
				"volume": liveDir.Name(),
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

	metadata := repo.liveMetadata(handle)
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
		Path:       repo.liveDataPath(handle),
		Properties: volumeProperties,
		TTL:        ttl,
		ExpiresAt:  expiresAt,
	}, nil
}

func (repo *repository) SetProperty(handle string, propertyName string, propertyValue string) error {
	repo.locker.Lock(handle)
	defer repo.locker.Unlock(handle)

	logger := repo.logger.Session("set-property")

	md := repo.liveMetadata(handle)

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

	err := repo.liveMetadata(handle).StoreTTL(TTL(ttl))
	if err != nil {
		logger.Error("failed-to-store-ttl", err)
		return ErrSetTTLFailed
	}

	return nil
}

func (repo *repository) VolumeParent(handle string) (Volume, bool, error) {
	parentDir, err := filepath.EvalSymlinks(filepath.Join(repo.liveMetadataPath(handle), "parent"))
	if os.IsNotExist(err) {
		return Volume{}, false, nil
	}

	if err != nil {
		return Volume{}, false, err
	}

	parentHandle := filepath.Base(parentDir)

	parentVolume, err := repo.GetVolume(parentHandle)
	if err != nil {
		return Volume{}, false, err
	}

	return parentVolume, true, nil
}

func (repo *repository) doStrategy(strategyName string, newVolumeMetadataPath string, newVolumeDataPath string, strategy Strategy, logger lager.Logger) error {
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

		parentDataPath := repo.liveDataPath(parentHandle)
		err := repo.createCowVolume(parentDataPath, newVolumeDataPath)
		if err != nil {
			logger.Error("failed-to-copy-volume", err)
			return ErrCreateVolumeFailed
		}

		err = os.Symlink(repo.liveMetadataPath(parentHandle), filepath.Join(newVolumeMetadataPath, "parent"))
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

func (repo *repository) initMetadataPath(id string) string {
	return filepath.Join(repo.initDir, id)
}

func (repo *repository) initDataPath(id string) string {
	return filepath.Join(repo.initDir, id, "volume")
}

func (repo *repository) initMetadata(id string) *Metadata {
	return &Metadata{path: repo.initMetadataPath(id)}
}

func (repo *repository) liveMetadataPath(id string) string {
	return filepath.Join(repo.liveDir, id)
}

func (repo *repository) liveDataPath(id string) string {
	return filepath.Join(repo.liveMetadataPath(id), "volume")
}

func (repo *repository) liveMetadata(id string) *Metadata {
	return &Metadata{path: repo.liveMetadataPath(id)}
}

func (repo *repository) deadMetadataPath(id string) string {
	return filepath.Join(repo.deadDir, id)
}

func (repo *repository) deadDataPath(id string) string {
	return filepath.Join(repo.deadDir, id, "volume")
}

func (repo *repository) cleanupFailedInitVolumeMetadataDir(id string) {
	err := os.RemoveAll(repo.initMetadataPath(id))
	if err != nil {
		repo.logger.Error("failed-to-cleanup", err, lager.Data{
			"volume": id,
		})
	}
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

func (repo *repository) volumeExists(handle string) bool {
	info, err := os.Stat(repo.liveMetadataPath(handle))
	if err != nil {
		return false
	}

	return info.IsDir()
}
