package baggageclaim

import (
	"encoding/json"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter . Client

// Client represents a client connection to a BaggageClaim server.
type Client interface {
	// CreateVolume will create a volume on the remote server. By passing in a
	// VolumeSpec with a different strategy you can choose the type of volume
	// that you want to create.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// CreateVolume returns the volume that was created or an error as to why it
	// could not be created.
	CreateVolume(lager.Logger, string, VolumeSpec) (Volume, error)

	// ListVolumes lists the volumes that are present on the server. A
	// VolumeProperties object can be passed in to filter the volumes that are in
	// the response.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// ListVolumes returns the volumes that were found or an error as to why they
	// could not be listed.
	ListVolumes(lager.Logger, VolumeProperties) (Volumes, error)

	// LookupVolume finds a volume that is present on the server. It takes a
	// string that corresponds to the Handle of the Volume.
	//
	// You are required to pass in a logger to the call to retain context across
	// the library boundary.
	//
	// LookupVolume returns a bool if the volume is found with the matching volume
	// or an error as to why the volume could not be found.
	LookupVolume(lager.Logger, string) (Volume, bool, error)
}

//go:generate counterfeiter . Volume

// Volume represents a volume in the BaggageClaim system.
type Volume interface {
	// Handle returns a per-server unique identifier for the volume. The URL of
	// the server and a handle is enough to universally identify a volume.
	Handle() string

	// Path returns the filesystem path to the volume on the server. This can be
	// supplied to other systems in order to let them use the volume.
	Path() string

	// SetTTL sets the volume's TTL to an absolute value. An error is returned if
	// the TTL could not be set.
	SetTTL(time.Duration) error

	// SetProperty sets a property on the Volume. Properties can be used to
	// filter the results in the ListVolumes call above.
	SetProperty(key string, value string) error

	// StreamIn calls BaggageClaim API endpoint in order to initialize tarStream
	// to stream the contents of the Reader into this volume at the specified path.
	StreamIn(path string, tarStream io.Reader) error

	StreamOut(path string) (io.ReadCloser, error)

	Expiration() (time.Duration, time.Time, error)

	// Properties returns the currently set properties for a Volume. An error is
	// returned if these could not be retrieved.
	Properties() (VolumeProperties, error)

	// Release stops the Volume being kept alive by the server. A final TTL can
	// be specified.
	Release(*time.Duration)

	// Size returns the exclusive size of the volume on disk in bytes
	SizeInBytes() (int64, error)

	Destroy() error
}

// Volumes represents a list of Volume object.
type Volumes []Volume

// VolumeProperties represents the properties for a particular volume.
type VolumeProperties map[string]string

// VolumeSpec is a specification representing the kind of volume that you'd
// like from the server.
type VolumeSpec struct {
	// Strategy is the information that the server requires to materialise the
	// volume. There are examples of these in this package.
	Strategy Strategy

	// Properties is the set of initial properties that the Volume should have.
	Properties VolumeProperties

	// TTL is the initial TTL of the volume.
	TTL time.Duration

	// Privileged is used to determine whether or not we need to perform a UID
	// translation of the files in the volume so that they can be read by a
	// non-privileged user.
	Privileged bool
}

type Strategy interface {
	Encode() *json.RawMessage
}

// ImportStrategy creates a volume by copying a directory from the host.
type ImportStrategy struct {
	// The location of the directory on the host to import.
	Path string
}

func (strategy ImportStrategy) Encode() *json.RawMessage {
	payload, _ := json.Marshal(struct {
		Type string `json:"type"`
		Path string `json:"path"`
	}{
		Type: "import",
		Path: strategy.Path,
	})

	msg := json.RawMessage(payload)
	return &msg
}

// COWStrategy creates a Copy-On-Write layer of another Volume.
type COWStrategy struct {
	// The parent volume that we should base the new volume on.
	Parent Volume
}

func (strategy COWStrategy) Encode() *json.RawMessage {
	payload, _ := json.Marshal(struct {
		Type   string `json:"type"`
		Volume string `json:"volume"`
	}{
		Type:   "cow",
		Volume: strategy.Parent.Handle(),
	})

	msg := json.RawMessage(payload)
	return &msg
}

// EmptyStrategy created a new empty volume.
type EmptyStrategy struct{}

func (EmptyStrategy) Encode() *json.RawMessage {
	msg := json.RawMessage(`{"type":"empty"}`)
	return &msg
}

func FinalTTL(dur time.Duration) *time.Duration {
	return &dur
}
