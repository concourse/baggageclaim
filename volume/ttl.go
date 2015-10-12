package volume

import "time"

type TTL uint

func (ttl TTL) Duration() time.Duration {
	return time.Duration(ttl) * time.Second
}

func (ttl TTL) IsUnlimited() bool {
	return ttl == 0
}
