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

const (
	StrategyEmpty       = "empty"
	StrategyCopyOnWrite = "cow"

	propertiesFileName = "properties.json"
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

type propertiesFile struct {
	path string
}

func (pf *propertiesFile) WriteProperties(properties Properties) error {
	file, err := os.OpenFile(
		pf.path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(properties)
}

func (pf *propertiesFile) Properties() (Properties, error) {
	var properties Properties

	file, err := os.Open(pf.path)
	if err != nil {
		return Properties{}, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&properties); err != nil {
		return Properties{}, err
	}

	return properties, nil
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

	err = repo.propertiesFile(guid).WriteProperties(properties)
	if err != nil {
		return repo.handleError(err, "failed-to-create-properties-file", ErrCreateVolumeFailed)
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
	volumeDirs, err := ioutil.ReadDir(repo.volumeDir)
	if err != nil {
		repo.logger.Error("failed-to-list-dirs", err, lager.Data{
			"volume-dir": repo.volumeDir,
		})

		return Volumes{}, ErrListVolumesFailed
	}

	response := make(Volumes, 0, len(volumeDirs))

	for _, volumeDir := range volumeDirs {
		volumeProperties, err := repo.propertiesFile(volumeDir.Name()).Properties()
		if err != nil {
			repo.logger.Error("failed-to-read-properties", err, lager.Data{
				"volume": volumeDir.Name(),
			})
			return nil, err
		}

		if volumeProperties.HasProperties(queryProperties) {
			response = append(response, Volume{
				GUID:       volumeDir.Name(),
				Path:       repo.dataPath(volumeDir.Name()),
				Properties: volumeProperties,
			})
		}
	}

	return response, nil
}

func (repo *Repository) SetProperty(volumeGUID string, propertyName string, propertyValue string) error {
	pf := repo.propertiesFile(volumeGUID)

	properties, err := pf.Properties()
	if err != nil {
		repo.logger.Error("failed-to-read-properties", err, lager.Data{
			"volume": volumeGUID,
		})
		return err
	}

	properties = properties.UpdateProperty(propertyName, propertyValue)

	err = pf.WriteProperties(properties)
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

func (repo *Repository) propertiesFile(id string) *propertiesFile {
	return &propertiesFile{path: repo.propertiesPath(id)}
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
