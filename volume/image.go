package volume

import (
	"context"
	"fmt"
	"io"

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

func NewImage(ctx context.Context, vol Volume) (i Image, err error) {
	switch {
	case IsDockerArchiveImage(vol):
		i, err = NewDockerArchiveVolumeImage(ctx, vol)
		if err != nil {
			err = fmt.Errorf("new docker archive image: %w", err)
			return
		}
	case IsConcourseImage(vol):
		i, err = NewConcourseVolumeImage(ctx, vol)
		if err != nil {
			err = fmt.Errorf("new concourse volume image: %w", err)
			return
		}
	default:
		err = fmt.Errorf("not a known image type")
		return
	}

	return
}
