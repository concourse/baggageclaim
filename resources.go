package baggageclaim

import (
	"encoding/json"
	"time"
)

type VolumeRequest struct {
	Handle       string           `json:"handle"`
	Strategy     *json.RawMessage `json:"strategy"`
	Properties   VolumeProperties `json:"properties"`
	TTLInSeconds uint             `json:"ttl,omitempty"`
	Privileged   bool             `json:"privileged,omitempty"`
}

type VolumeResponse struct {
	Handle       string           `json:"handle"`
	Path         string           `json:"path"`
	Properties   VolumeProperties `json:"properties"`
	TTLInSeconds uint             `json:"ttl,omitempty"`
	ExpiresAt    time.Time        `json:"expires_at"`
}

type VolumeStatsResponse struct {
	SizeInBytes int64 `json:"size_in_bytes"`
}

type PropertyRequest struct {
	Value string `json:"value"`
}

type TTLRequest struct {
	Value uint `json:"value"`
}
