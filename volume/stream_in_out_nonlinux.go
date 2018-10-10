// +build !linux

package volume

import (
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/go-archive/tgzfs"
)

func (streamer *streamer) In(stream io.Reader, dest string, privileged bool) (bool, error) {
	err := tgzfs.Extract(stream, dest)
	if err != nil {
		return true, err
	}

	return false, nil
}

func (streamer *streamer) Out(w io.Writer, src string, privileged bool) error {
	fileInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	var tarDir, tarPath string

	if fileInfo.IsDir() {
		tarDir = src
		tarPath = "."
	} else {
		tarDir = filepath.Dir(src)
		tarPath = filepath.Base(src)
	}

	return tgzfs.Compress(w, tarDir, tarPath)
}
