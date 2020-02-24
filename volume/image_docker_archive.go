package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/containers/image/v5/docker/archive"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerArchiveVolumeImage struct {
	vol Volume
	src types.ImageSource
}

func IsDockerArchiveImage(vol Volume) bool {
	fpath := filepath.Join(vol.Path, "image.tar")

	_, err := os.Stat(fpath)
	return err == nil
}

func NewDockerArchiveVolumeImage(ctx context.Context, vol Volume) (i *dockerArchiveVolumeImage, err error) {
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

func (i dockerArchiveVolumeImage) GetManifest(ctx context.Context) (b []byte, desc imgspecv1.Descriptor, err error) {
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

func (i dockerArchiveVolumeImage) GetBlob(ctx context.Context, d string) (blob io.ReadCloser, size int64, err error) {
	blob, size, err = i.src.GetBlob(ctx, types.BlobInfo{
		Digest: digest.Digest(d),
	}, nil)
	if err != nil {
		err = fmt.Errorf("get blob: %w", err)
		return
	}

	return
}

func (i dockerArchiveVolumeImage) Close() error {
	return i.src.Close()
}
