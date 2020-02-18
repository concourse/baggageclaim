package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
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

	i = &tarballVolumeImage{
		vol: vol,
		src: src,
	}

	return
}

type tarballVolumeImage struct {
	vol Volume
	src types.ImageSource
}

func (i tarballVolumeImage) GetManifest(ctx context.Context) (b []byte, desc imgspecv1.Descriptor, err error) {
	b, _, err = i.src.GetManifest(ctx, nil)
	if err != nil {
		err = fmt.Errorf("get manifest: %w", err)
		return
	}

	schema2 := manifest.Schema2{}
	err = json.Unmarshal(b, &schema2)
	if err != nil {
		err = fmt.Errorf("manifest to schema2 unmarshal: %w", err)
		return
	}

	schema2.MediaType = manifest.DockerV2Schema2MediaType

	b, err = json.Marshal(&schema2)
	if err != nil {
		err = fmt.Errorf("serializing schema2: %w", err)
		return
	}

	desc = imgspecv1.Descriptor{
		Size:      int64(len(b)),
		Digest:    digest.FromBytes(b),
		MediaType: schema2.MediaType,
	}
	return
}

func (i tarballVolumeImage) GetBlob(ctx context.Context, d string) (blob io.ReadCloser, size int64, err error) {
	blob, size, err = i.src.GetBlob(ctx, types.BlobInfo{
		Digest: digest.Digest(d),
	}, nil)
	if err != nil {
		err = fmt.Errorf("get blob: %w", err)
		return
	}

	return
}

func (i tarballVolumeImage) Close() error {
	return i.src.Close()
}
