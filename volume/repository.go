package volume

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/nu7hatch/gouuid"
	"github.com/pivotal-golang/lager"
)

type Strategy map[string]string

type Properties map[string]string

func (p Properties) HasProperties(other Properties) bool {
	if len(other) > len(p) {
		return false
	}

	for otherName, otherValue := range other {
		value, found := p[otherName]
		if !found || value != otherValue {
			return false
		}
	}

	return true
}

const (
	StrategyEmpty       = "empty"
	StrategyCopyOnWrite = "cow"
	propertiesFileName  = "properties.json"
)

type Volume struct {
	GUID       string     `json:"guid"`
	Path       string     `json:"path"`
	Properties Properties `json:"properties"`
}

type Volumes []Volume

var ErrMissingStrategy = errors.New("missing strategy")
var ErrUnrecognizedStrategy = errors.New("unrecognized strategy")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrSetPropertyFailed = errors.New("failed to set property on volume")
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

func (repo *Repository) CreateVolume(strategy Strategy, properties Properties) (Volume, error) {
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
		return repo.handleError(err, "failed-to-create-metadata-dir", ErrCreateVolumeFailed)
	}

	propertiesBytes, err := json.Marshal(properties)
	if err != nil {
		return repo.handleError(err, "failed-to-marshal-properties", ErrCreateVolumeFailed)
	}

	err = ioutil.WriteFile(repo.propertiesPath(guid), propertiesBytes, 0644)
	if err != nil {
		return repo.handleError(err, "failed-to-write-properties", ErrCreateVolumeFailed)
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
	}, nil
}

func (repo *Repository) handleError(internalError error, errorMsg string, externalError error) (Volume, error) {
	repo.logger.Error(errorMsg, internalError)
	return Volume{}, externalError
}

func (repo *Repository) ListVolumes(queryProperties Properties) (Volumes, error) {
	volumes, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		repo.logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumes))
	for _, volume := range volumes {
		propertiesPath := repo.propertiesPath(volume.Name())
		propBytes, err := ioutil.ReadFile(propertiesPath)
		if err != nil {
			repo.logger.Error("failed-to-read-properties", err, lager.Data{
				"volume": volume.Name(),
			})
			return nil, err
		}

		var properties Properties
		err = json.Unmarshal(propBytes, &properties)
		if err != nil {
			repo.logger.Error("failed-to-unmarshal-properties", err, lager.Data{
				"volume": volume.Name(),
			})
			return nil, err
		}

		if properties.HasProperties(queryProperties) {
			response = append(response, Volume{
				GUID:       volume.Name(),
				Path:       repo.dataPath(volume.Name()),
				Properties: properties,
			})
		}
	}

	return response, nil
}

func (repo *Repository) SetProperty(volumeGUID string, propertyName string, propertyValue string) error {
	propertiesPath := repo.propertiesPath(volumeGUID)

	propBytes, err := ioutil.ReadFile(propertiesPath)
	if err != nil {
		_, err = repo.handleError(err, "failed-to-read-properties-file", ErrSetPropertyFailed)
		return err
	}

	var properties Properties
	err = json.Unmarshal(propBytes, &properties)
	if err != nil {
		repo.logger.Error("failed-to-unmarshal-properties", err, lager.Data{
			"volume": volumeGUID,
		})
		return err
	}

	properties[propertyName] = propertyValue

	propertiesBytes, err := json.Marshal(properties)
	if err != nil {
		_, err = repo.handleError(err, "failed-to-marshal-properties", ErrSetPropertyFailed)
		return err
	}

	err = ioutil.WriteFile(repo.propertiesPath(volumeGUID), propertiesBytes, 0644)
	if err != nil {
		_, err = repo.handleError(err, "failed-to-write-properties", ErrSetPropertyFailed)
		return err
	}

	return nil
}

func (repo *Repository) doStrategy(strategyName string, newVolumeDataPath string, strategy Strategy, logger lager.Logger) error {
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

func (repo *Repository) metadataPath(id string) string {
	return filepath.Join(repo.volumeDir, id)
}

func (repo *Repository) propertiesPath(id string) string {
	return filepath.Join(repo.metadataPath(id), propertiesFileName)
}

func (repo *Repository) dataPath(id string) string {
	return filepath.Join(repo.metadataPath(id), "volume")
}

func (repo *Repository) deleteVolumeMetadataDir(id string) {
	err := os.RemoveAll(repo.metadataPath(id))
	if err != nil {
		repo.logger.Error("failed-to-cleanup", err, lager.Data{
			"guid": id,
		})
	}
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
