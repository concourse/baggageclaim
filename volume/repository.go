package volume

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/lager"
)

type Strategy map[string]string

const (
	StrategyEmpty       = "empty"
	StrategyCopyOnWrite = "cow"
)

type Volume struct {
	GUID string `json:"guid"`
	Path string `json:"path"`
}

type Volumes []Volume

var ErrMissingStrategy = errors.New("missing strategy")
var ErrUnrecognizedStrategy = errors.New("unrecognized strategy")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrNoParentVolumeProvided = errors.New("no parent volume provided")
var ErrParentVolumeNotFound = errors.New("parent volume not found")

type Repository struct {
	volumeDir string
	driver    Driver

	logger lager.Logger
}

func NewRepository(logger lager.Logger, volumeDir string, driver Driver) *Repository {
	return &Repository{
		volumeDir: volumeDir,
		logger:    logger,
		driver:    driver,
	}
}

type Driver interface {
	CreateVolume(path string) error
	CreateCopyOnWriteLayer(path string, parent string) error
}

func (repo *Repository) CreateVolume(strategy Strategy) (Volume, error) {
	strategyName, found := strategy["type"]
	if !found {
		return Volume{}, ErrMissingStrategy
	}

	logger := repo.logger.Session("create-volume", lager.Data{
		"strategy": strategyName,
	})

	guid := repo.createUuid()
	newVolumeMetadataPath := repo.metadataPath(guid)
	err := os.Mkdir(newVolumeMetadataPath, 0755)
	if err != nil {
		logger.Error("failed-to-create-metadata-dir", err)
		return Volume{}, ErrCreateVolumeFailed
	}

	newVolumeDataPath := repo.dataPath(guid)

	switch strategyName {
	case StrategyEmpty:
		err := repo.createEmptyVolume(newVolumeDataPath)
		if err != nil {
			logger.Error("failed-to-create-volume", err, lager.Data{
				"path": newVolumeDataPath,
			})
			repo.deleteVolumeMetadataDir(guid)
			return Volume{}, ErrCreateVolumeFailed
		}

	case StrategyCopyOnWrite:
		parentGUID, found := strategy["volume"]
		if !found {
			logger.Error("no-parent-volume-provided", nil)
			repo.deleteVolumeMetadataDir(guid)
			return Volume{}, ErrNoParentVolumeProvided
		}

		if !repo.volumeExists(parentGUID) {
			logger.Error("parent-volume-not-found", nil)
			repo.deleteVolumeMetadataDir(guid)
			return Volume{}, ErrParentVolumeNotFound
		}

		parentDataPath := repo.dataPath(parentGUID)
		err := repo.createCowVolume(parentDataPath, newVolumeDataPath)
		if err != nil {
			logger.Error("failed-to-copy-volume", err)
			repo.deleteVolumeMetadataDir(guid)
			return Volume{}, ErrCreateVolumeFailed
		}

	default:
		logger.Error("unrecognized-strategy", nil, lager.Data{
			"strategy": strategyName,
		})
		repo.deleteVolumeMetadataDir(guid)
		return Volume{}, ErrUnrecognizedStrategy
	}

	return Volume{
		Path: newVolumeDataPath,
		GUID: guid,
	}, nil
}

func (repo *Repository) ListVolumes() (Volumes, error) {
	volumes, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		repo.logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumes))
	for _, volume := range volumes {
		response = append(response, Volume{
			GUID: volume.Name(),
			Path: repo.dataPath(volume.Name()),
		})
	}

	return response, nil
}

func (repo *Repository) metadataPath(id string) string {
	return filepath.Join(repo.volumeDir, id)
}

func (repo *Repository) dataPath(id string) string {
	return filepath.Join(repo.metadataPath(id), "volume")
}

func (repo *Repository) deleteVolumeMetadataDir(id string) error {
	return os.RemoveAll(repo.metadataPath(id))
}

func (repo *Repository) createEmptyVolume(volumePath string) error {
	err := repo.driver.CreateVolume(volumePath)
	if err != nil {
		return err
	}

	return nil
}

func (repo *Repository) createCowVolume(parentPath string, newPath string) error {
	err := repo.driver.CreateCopyOnWriteLayer(newPath, parentPath)
	if err != nil {
		return err
	}

	return nil
}

func (repo *Repository) createUuid() string {
	guid, err := uuid.NewV4()
	if err != nil {
		repo.logger.Fatal("failed-to-generate-guid", err)
	}

	return guid.String()
}

func (repo *Repository) volumeExists(guid string) bool {
	info, err := os.Stat(repo.metadataPath(guid))
	if err != nil {
		return false
	}

	return info.IsDir()
}
