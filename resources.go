package baggageclaim

import (
	"encoding/json"
)

type VolumeRequest struct {
	Handle     string           `json:"handle"`
	Strategy   *json.RawMessage `json:"strategy"`
	Properties VolumeProperties `json:"properties"`
	Privileged bool             `json:"privileged,omitempty"`
}

// StreamToRequest represents the payload to be sent by someone wanting to get
// this baggageclaim server to stream a given volume to a differente worker.
//
type StreamToRequest struct {
	// Destination is the URL of the baggageclaim that is going to be
	// targetted at.
	//
	Destination string `json:"destination"`

	// Handle is the handle of the volume that must exist on the destination
	// before we start streaming to it.
	//
	Handle string `json:"handle"`

	// Path is the path inside the volume where the content is going to be
	// streamed to.
	//
	Path string `json:"path"`
}

type VolumeResponse struct {
	Handle     string           `json:"handle"`
	Path       string           `json:"path"`
	Properties VolumeProperties `json:"properties"`
}

type VolumeFutureResponse struct {
	Handle string `json:"handle"`
}

type PropertyRequest struct {
	Value string `json:"value"`
}

type PrivilegedRequest struct {
	Value bool `json:"value"`
}
