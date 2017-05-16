package volume

import "time"

type Volume struct {
	Handle     string     `json:"handle"`
	Path       string     `json:"path"`
	Properties Properties `json:"properties"`
	TTL        TTL        `json:"ttl,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	Privileged bool       `json:"privileged"`
}

type Volumes []Volume
