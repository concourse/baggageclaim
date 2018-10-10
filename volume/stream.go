package volume

import (
	"io"

	"github.com/concourse/baggageclaim/uidgid"
)

//go:generate counterfeiter . Streamer

type Streamer interface {
	In(io.Reader, string, bool) (bool, error)
	Out(io.Writer, string, bool) error
}

type streamer struct {
	namespacer uidgid.Namespacer
}
