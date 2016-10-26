package volume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type VolumeState string

const (
	propertiesFileName   = "properties.json"
	ttlFileName          = "ttl.json"
	isPrivilegedFileName = "privileged.json"
)

type Metadata struct {
	path string
}

// Properties File
func (md *Metadata) Properties() (Properties, error) {
	return md.propertiesFile().Properties()
}

func (md *Metadata) StoreProperties(properties Properties) error {
	return md.propertiesFile().WriteProperties(properties)
}

func (md *Metadata) propertiesFile() *propertiesFile {
	return &propertiesFile{path: filepath.Join(md.path, propertiesFileName)}
}

type propertiesFile struct {
	path string
}

func (pf *propertiesFile) WriteProperties(properties Properties) error {
	return writeMetadataFile(pf.path, properties)
}

func (pf *propertiesFile) Properties() (Properties, error) {
	var properties Properties

	err := readMetadataFile(pf.path, &properties)
	if err != nil {
		return Properties{}, err
	}

	return properties, nil
}

// TTL File
func (md *Metadata) TTL() (TTL, time.Time, error) {
	properties, err := md.ttlFile().Properties()
	if err != nil {
		return 0, time.Time{}, err
	}

	return properties.TTL, time.Unix(properties.ExpiresAt, 0), nil
}

func (md *Metadata) StoreTTL(ttl TTL) (time.Time, error) {
	return md.ttlFile().WriteTTL(ttl)
}

func (md *Metadata) isPrivilegedFile() *isPrivilegedFile {
	return &isPrivilegedFile{path: filepath.Join(md.path, isPrivilegedFileName)}
}

func (md *Metadata) IsPrivileged() (bool, error) {
	return md.isPrivilegedFile().IsPrivileged()
}

func (md *Metadata) StorePrivileged(isPrivileged bool) error {
	return md.isPrivilegedFile().WritePrivileged(isPrivileged)
}

func (md *Metadata) ExpiresAt() (time.Time, error) {
	properties, err := md.ttlFile().Properties()
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(properties.ExpiresAt, 0), nil
}

func (md *Metadata) ttlFile() *ttlFile {
	return &ttlFile{path: filepath.Join(md.path, ttlFileName)}
}

type ttlFile struct {
	path string
}

type ttlProperties struct {
	TTL       TTL   `json:"ttl"`
	ExpiresAt int64 `json:"expires_at"`
}

func (tf *ttlFile) WriteTTL(ttl TTL) (time.Time, error) {
	expiresAt := time.Now().Add(ttl.Duration()).Unix()

	err := writeMetadataFile(tf.path, ttlProperties{
		TTL:       ttl,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(expiresAt, 0), nil
}

func (tf *ttlFile) Properties() (ttlProperties, error) {
	var properties ttlProperties
	err := readMetadataFile(tf.path, &properties)
	if err != nil {
		return ttlProperties{}, err
	}

	return properties, nil
}

type isPrivilegedFile struct {
	path string
}

func (ipf *isPrivilegedFile) WritePrivileged(isPrivileged bool) error {
	return writeMetadataFile(ipf.path, isPrivileged)
}

func (ipf *isPrivilegedFile) IsPrivileged() (bool, error) {
	var isPrivileged bool

	err := readMetadataFile(ipf.path, &isPrivileged)
	if err != nil {
		return false, err
	}

	return isPrivileged, nil
}

func readMetadataFile(path string, properties interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return ErrVolumeDoesNotExist
		}

		return err
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&properties); err != nil {
		return err
	}

	return nil
}

func writeMetadataFile(path string, properties interface{}) error {
	file, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		if _, ok := err.(*os.PathError); ok {
			return ErrVolumeDoesNotExist
		}

		return err
	}

	defer file.Close()

	return json.NewEncoder(file).Encode(properties)
}
