package volume

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/pivotal-golang/lager"
	"golang.org/x/net/context"
)

type DockerImageStrategy struct {
	LockManager LockManager

	RegistryURL string
	Repository  string
	Username    string
	Password    string
	Tag         string
}

func (strategy DockerImageStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	transport, err := strategy.makeTransport(logger)
	if err != nil {
		logger.Error("failed-to-construct-transport", err)
		return nil, err
	}

	repoClient, err := client.NewRepository(context.TODO(), strategy.Repository, strategy.RegistryURL, transport)
	if err != nil {
		logger.Error("failed-to-construct-repository", err)
		return nil, err
	}

	blobStore := repoClient.Blobs(context.TODO())

	manifestService, err := repoClient.Manifests(context.TODO())
	if err != nil {
		logger.Error("failed-to-get-manifests", err)
		return nil, err
	}

	tagManifest, err := manifestService.GetByTag(strategy.Tag)
	if err != nil {
		logger.Error("failed-to-get-tag", err)
		return nil, err
	}

	var parent FilesystemLiveVolume
	for i := len(tagManifest.FSLayers) - 1; i >= 0; i-- {
		layer := tagManifest.FSLayers[i]

		var image imageJSON
		err := json.Unmarshal([]byte(tagManifest.History[i].V1Compatibility), &image)
		if err != nil {
			logger.Error("malformed-image", err)
			return nil, err
		}

		parent, err = strategy.fetchDockerLayer(
			logger.Session("fetch", lager.Data{"image": image.ID}),
			fs,
			blobStore,
			image,
			layer.BlobSum,
		)
		if err != nil {
			logger.Error("failed-to-fetch-layer", err)
			return nil, err
		}
	}

	return parent.NewSubvolume(handle)
}

func (strategy DockerImageStrategy) makeTransport(logger lager.Logger) (http.RoundTripper, error) {
	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		DisableKeepAlives: true,
	}

	authTransport := transport.NewTransport(baseTransport)

	pingClient := &http.Client{
		Transport: authTransport,
		Timeout:   5 * time.Second,
	}

	req, err := http.NewRequest("GET", strategy.RegistryURL+"/v2", nil)
	if err != nil {
		logger.Error("failed-to-create-ping-request", err)
		return nil, err
	}

	challengeManager := auth.NewSimpleChallengeManager()

	resp, err := pingClient.Do(req)
	if err != nil {
		logger.Error("failed-to-ping-registry", err)
		return nil, err
	} else {
		defer resp.Body.Close()

		if err := challengeManager.AddResponse(resp); err != nil {
			logger.Error("failed-to-add-response-to-challenge-manager", err)
			return nil, err
		}
	}

	credentialStore := dumbCredentialStore{strategy.Username, strategy.Password}
	tokenHandler := auth.NewTokenHandler(authTransport, credentialStore, strategy.Repository, "pull")
	basicHandler := auth.NewBasicHandler(credentialStore)
	authorizer := auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler)

	return transport.NewTransport(baseTransport, authorizer), nil
}

func (strategy DockerImageStrategy) fetchDockerLayer(
	logger lager.Logger,
	fs Filesystem,
	blobStore distribution.BlobStore,
	image imageJSON,
	digest digest.Digest,
) (FilesystemLiveVolume, error) {
	handle := image.ID
	if handle == "" {
		logger.Error("empty-image-id", nil)
		return nil, errors.New("empty image id")
	}

	strategy.LockManager.Lock(handle)
	defer strategy.LockManager.Unlock(handle)

	existing, found, err := fs.LookupVolume(handle)
	if err != nil {
		logger.Error("failed-to-check-for-existing-volume", err)
		return nil, err
	}

	if found {
		logger.Info("already-fetched")
		return existing, nil
	}

	tgzReader, err := blobStore.Open(context.TODO(), digest)
	if err != nil {
		logger.Error("failed-to-open-layer-blob", err)
		return nil, err
	}

	defer tgzReader.Close()

	parentHandle := image.Parent

	var initLayerVolume FilesystemInitVolume
	if parentHandle != "" {
		logger.Debug("has-parent", lager.Data{"parent": parentHandle})

		parent, found, err := fs.LookupVolume(parentHandle)
		if err != nil {
			logger.Error("failed-to-look-for-parent", err)
			return nil, err
		}

		if !found {
			logger.Error("parent-volume-not-found", nil)
			return nil, ErrParentVolumeNotFound
		}

		initLayerVolume, err = parent.NewSubvolume(handle)
	} else {
		logger.Debug("first-layer")
		initLayerVolume, err = fs.NewVolume(handle)
	}

	if err != nil {
		logger.Error("failed-to-create-layer-volume", err)
		return nil, err
	}

	var initialized bool
	defer func() {
		if !initialized {
			initLayerVolume.Destroy()
		}
	}()

	err = strategy.extract(logger.Session("extract"), initLayerVolume, tgzReader)
	if err != nil {
		return nil, err
	}

	layerVolume, err := initLayerVolume.Initialize()
	if err != nil {
		return nil, err
	}

	initialized = true

	logger.Info("initialized")

	return layerVolume, nil
}

func (strategy DockerImageStrategy) extract(
	logger lager.Logger,
	volume FilesystemInitVolume,
	tgzReader io.Reader,
) error {
	logger.Debug("start")
	defer logger.Debug("done")

	extract := exec.Command("tar", "zxf", "-", "-C", volume.DataPath())
	extract.Stdin = tgzReader

	output, err := extract.CombinedOutput()
	if err != nil {
		logger.Error("failed-to-extract", err, lager.Data{"output": string(output)})
		return fmt.Errorf("extract failed: %s (output: %v)", err, string(output))
	}

	return nil
}

type dumbCredentialStore struct {
	username string
	password string
}

func (dcs dumbCredentialStore) Basic(*url.URL) (string, string) {
	return dcs.username, dcs.password
}

// NB: this has a ton of other stuff that we don't need yet, so their fields
// are omitted
type imageJSON struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
}
