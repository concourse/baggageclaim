package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

type dockerArchiveVolumeImage struct {
	vol Volume
	src types.ImageSource
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
