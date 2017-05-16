// +build !linux

package api

import (
	"io"
	"os"
	"path/filepath"

	"github.com/concourse/go-archive/tarfs"
)

func (vs *VolumeServer) streamIn(stream io.Reader, dest string, privileged bool) (bool, error) {
	err := tarfs.Extract(stream, dest)
	if err != nil {
		return true, err
	}

	return false, nil
}

func (vs *VolumeServer) streamOut(w io.Writer, src string, privileged bool) error {
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

	return tarfs.Compress(w, tarDir, tarPath)
}
