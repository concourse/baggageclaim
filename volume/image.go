package volume

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containers/image/v5/docker/archive"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Image
//
type Image interface {
	io.Closer

	// GetManifest retrieves a reader that brings back the OCI image spec
	// manifest associated with a volume that holds an OCI image.
	//
	GetManifest(ctx context.Context) (b []byte, desc imgspecv1.Descriptor, err error)

	// GetBlob retrieves a reader that when read, gives the contents of a
	// blob.
	//
	GetBlob(ctx context.Context, digest string) (blob io.ReadCloser, size int64, err error)
}

// TODO detect if we're dealing with OCI layout, or with a plain `tarball`
// extracted image.
//
func NewImage(ctx context.Context, vol Volume) (i Image, err error) {
	tarballPath := filepath.Join(vol.Path, "image.tar")

	ref, err := archive.NewReference(tarballPath, nil)
	if err != nil {
		err = fmt.Errorf("new ref: %w", err)
		return
	}

	src, err := ref.NewImageSource(ctx, nil)
	if err != nil {
		err = fmt.Errorf("new image source: %w", err)
		return
	}

	i = &dockerArchiveVolumeImage{
		vol: vol,
		src: src,
	}

	return
}
