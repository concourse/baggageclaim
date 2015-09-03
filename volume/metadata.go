package volume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type VolumeState string

const (
	propertiesFileName = "properties.json"
	ttlFileName        = "ttl.json"
	stateFileName      = "state.json"

	VolumeDestroyed VolumeState = "destroyed"
	VolumeActive    VolumeState = "active"
	CreatingVolume  VolumeState = "creating"
)

type Metadata struct {
	path string
}

func (md *Metadata) Properties() (Properties, error) {
	return md.propertiesFile().Properties()
}

func (md *Metadata) StoreProperties(properties Properties) error {
	return md.propertiesFile().WriteProperties(properties)
}

func (md *Metadata) VolumeState() (VolumeState, error) {
	volumeStateProperties, err := md.stateFile().Properties()
	if err != nil {
		return "", err
	}
	return volumeStateProperties.State, nil
}

func (md *Metadata) StoreState(volumeState VolumeState) error {
	return md.stateFile().WriteState(volumeState)
}

func (md *Metadata) stateFile() *stateFile {
	return &stateFile{path: filepath.Join(md.path, stateFileName)}
}

type stateFile struct {
	path string
}

type stateProperties struct {
	State VolumeState `json:"state"`
}

func (sf *stateFile) Properties() (stateProperties, error) {
	var properties stateProperties

	file, err := os.Open(sf.path)
	if err != nil {
		return stateProperties{}, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&properties); err != nil {
		return stateProperties{}, err
	}

	return properties, nil
}

func (sf *stateFile) WriteState(volumeState VolumeState) error {
	file, err := os.OpenFile(
		sf.path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	properties := stateProperties{
		State: volumeState,
	}

	return json.NewEncoder(file).Encode(properties)
}

func (md *Metadata) StoreTTL(ttl TTL) error {
	return md.ttlFile().WriteTTL(ttl)
}

func (md *Metadata) TTL() (TTL, error) {
	properties, err := md.ttlFile().Properties()
	if err != nil {
		return TTL(0), err
	}

	return properties.TTL, nil
}

func (md *Metadata) ExpiresAt() (time.Time, error) {
	properties, err := md.ttlFile().Properties()
	if err != nil {
		return time.Time{}, err
	}

	return properties.ExpiresAt, nil
}

func (md *Metadata) ttlFile() *ttlFile {
	return &ttlFile{path: filepath.Join(md.path, ttlFileName)}
}

type ttlFile struct {
	path string
}

type ttlProperties struct {
	TTL       TTL       `json:"ttl"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (tf *ttlFile) WriteTTL(ttl TTL) error {
	file, err := os.OpenFile(
		tf.path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer file.Close()

	properties := ttlProperties{
		TTL:       ttl,
		ExpiresAt: time.Now().Add(ttl.Duration()),
	}

	return json.NewEncoder(file).Encode(properties)
}

func (tf *ttlFile) Properties() (ttlProperties, error) {
	var properties ttlProperties

	file, err := os.Open(tf.path)
	if err != nil {
		return ttlProperties{}, err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&properties); err != nil {
		return ttlProperties{}, err
	}

	return properties, nil
}

func (md *Metadata) propertiesFile() *propertiesFile {
	return &propertiesFile{path: filepath.Join(md.path, propertiesFileName)}
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
