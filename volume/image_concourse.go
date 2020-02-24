package volume

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/go-archive/tarfs"
	"github.com/containers/image/v5/manifest"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Image
//
type concourseVolumeImage struct {
	vol            Volume
	manifestDigest string
}

// IsConcurseImage verifies if the volume hosts a "concourse" container image.
//
// ref: https://concourse-ci.org/tasks.html#image_resource
//
func IsConcourseImage(vol Volume) bool {
	rootfsPath := filepath.Join(vol.Path, "rootfs")
	metadataPath := filepath.Join(vol.Path, "metadata.json")

	_, err := os.Stat(rootfsPath)
	if err != nil {
		return false
	}

	_, err = os.Stat(metadataPath)
	if err != nil {
		return false
	}

	return true
}

func NewConcourseVolumeImage(ctx context.Context, vol Volume) (i *concourseVolumeImage, err error) {
	i = &concourseVolumeImage{
		vol: vol,
	}

	err = i.setupImage(vol.Path)
	if err != nil {
		err = fmt.Errorf("setup image: %w", err)
		return
	}

	return
}

func (i concourseVolumeImage) Close() error {
	return nil
}

func (i concourseVolumeImage) GetManifest(ctx context.Context) (b []byte, desc imgspecv1.Descriptor, err error) {
	fpath := filepath.Join(i.vol.Path, i.manifestDigest)

	f, err := os.Open(fpath)
	if err != nil {
		err = fmt.Errorf("open %s: %w", fpath, err)
		return
	}

	defer f.Close()

	finfo, err := f.Stat()
	if err != nil {
		err = fmt.Errorf("stat: %w", err)
		return
	}

	b, err = ioutil.ReadAll(f)
	if err != nil {
		err = fmt.Errorf("read all: %w", err)
		return
	}

	desc.Digest = digest.Digest(i.manifestDigest)
	desc.MediaType = manifest.DockerV2Schema2MediaType
	desc.Size = finfo.Size()

	return
}

func (i concourseVolumeImage) GetBlob(ctx context.Context, digest string) (blob io.ReadCloser, size int64, err error) {
	fpath := filepath.Join(i.vol.Path, digest)

	f, err := os.Open(fpath)
	if err != nil {
		err = fmt.Errorf("open %s: %w", fpath, err)
		return
	}

	finfo, err := f.Stat()
	if err != nil {
		f.Close()
		err = fmt.Errorf("stat: %w", err)
		return
	}

	size = finfo.Size()
	blob = io.ReadCloser(f)
	return
}

func (i *concourseVolumeImage) isAlreadySetup(dir string) bool {
	fpath := filepath.Join(dir, "manifest.json")

	tgt, err := os.Readlink(fpath)
	if err != nil {
		return false
	}

	i.manifestDigest = filepath.Base(tgt)
	return true

}

func (i *concourseVolumeImage) setupImage(dir string) (err error) {
	if i.isAlreadySetup(dir) {
		return
	}

	imgConfig, err := MetadataToImageConfig(dir)
	if err != nil {
		err = fmt.Errorf("metadata to image config: %w", err)
		return
	}

	// TODO do this only if necessary
	rootfsPath, err := RootfsArchive(dir)
	if err != nil {
		err = fmt.Errorf("create rootfs layer: %w", err)
		return
	}

	// do this at the same time?
	//
	layerDesc, err := CreateLayerDescriptor(rootfsPath)
	if err != nil {
		err = fmt.Errorf("create layer descriptor: %w", err)
		return
	}

	err = os.Rename(rootfsPath, filepath.Join(filepath.Dir(rootfsPath), layerDesc.Digest.String()))
	if err != nil {
		err = fmt.Errorf("rename rootfs to blob digest: %w", err)
		return
	}

	rootfs := manifest.Schema2RootFS{
		Type:    imgspecv1.MediaTypeImageLayer,
		DiffIDs: []digest.Digest{layerDesc.Digest},
	}

	config := manifest.Schema2Image{
		Schema2V1Image: manifest.Schema2V1Image{
			Architecture:    runtime.GOARCH,
			OS:              runtime.GOOS,
			Config:          &imgConfig,
			ContainerConfig: imgConfig,
		},
		RootFS: &rootfs,
	}

	configB, err := json.Marshal(config)
	if err != nil {
		err = fmt.Errorf("config marshal: %w", err)
		return
	}

	configDigest := digest.FromBytes(configB)
	configFpath := filepath.Join(dir, configDigest.String())

	err = ioutil.WriteFile(configFpath, configB, 0777)
	if err != nil {
		err = fmt.Errorf("write file: %w", err)
		return
	}

	specManifest := manifest.Schema2{
		MediaType:     manifest.DockerV2Schema2MediaType,
		SchemaVersion: 2,
		ConfigDescriptor: manifest.Schema2Descriptor{
			Digest:    configDigest,
			Size:      int64(len(configB)),
			MediaType: imgspecv1.MediaTypeImageConfig,
		},
		LayersDescriptors: []manifest.Schema2Descriptor{
			{
				MediaType: layerDesc.MediaType,
				Size:      layerDesc.Size,
				Digest:    layerDesc.Digest,
			},
		},
	}

	manifestB, err := json.Marshal(specManifest)
	if err != nil {
		err = fmt.Errorf("manifest marshal: %w", err)
		return
	}

	manifestDigest := digest.FromBytes(manifestB)
	manifestAliasFpath := filepath.Join(dir, "manifest.json")
	manifestFpath := filepath.Join(dir, manifestDigest.String())

	err = ioutil.WriteFile(manifestFpath, manifestB, 0777)
	if err != nil {
		err = fmt.Errorf("write file: %w", err)
		return
	}

	err = os.Symlink(manifestFpath, manifestAliasFpath)
	if err != nil {
		err = fmt.Errorf("symlink: %w", err)
		return
	}

	i.manifestDigest = manifestDigest.String()

	return
}

func CreateLayerDescriptor(fpath string) (desc manifest.Schema2Descriptor, err error) {
	f, err := os.Open(fpath)
	if err != nil {
		err = fmt.Errorf("open %s: %w", fpath, err)
		return
	}
	defer f.Close()

	d, err := digest.FromReader(f)
	if err != nil {
		err = fmt.Errorf("digest: %w", err)
		return
	}

	finfo, err := f.Stat()
	if err != nil {
		err = fmt.Errorf("stat: %w", err)
		return
	}

	desc = manifest.Schema2Descriptor{
		Digest:    d,
		Size:      finfo.Size(),
		MediaType: imgspecv1.MediaTypeImageLayer,
	}

	return
}

type ImageMetadata struct {
	Env  []string `json:"env"`
	User string   `json:"user"`
}

func MetadataToImageConfig(dir string) (cfg manifest.Schema2Config, err error) {
	fpath := filepath.Join(dir, "metadata.json")

	f, err := os.Open(fpath)
	if err != nil {
		err = fmt.Errorf("open %s: %w", fpath, err)
		return
	}
	defer f.Close()

	decoder := json.NewDecoder(f)

	metadata := ImageMetadata{}
	err = decoder.Decode(&metadata)
	if err != nil {
		err = fmt.Errorf("decode: %w", err)
		return
	}

	cfg = manifest.Schema2Config{
		Env:  metadata.Env,
		User: metadata.User,
	}

	return
}

func RootfsArchive(dir string) (result string, err error) {
	result = filepath.Join(dir, "rootfs.tar")

	f, err := os.Create(result)
	if err != nil {
		err = fmt.Errorf("create: %w", err)
		return
	}
	defer f.Close()

	err = tarfs.Compress(f, filepath.Join(dir, "rootfs"), ".")
	if err != nil {
		err = fmt.Errorf("compress: %w", err)
		return
	}

	return
}
