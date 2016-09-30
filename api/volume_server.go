package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/tedsuo/rata"
)

const httpUnprocessableEntity = 422

var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrGetVolumeFailed = errors.New("failed to get volume")
var ErrGetVolumeStatsFailed = errors.New("failed to get volume stats")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrDestroyVolumeFailed = errors.New("failed to destroy volume")
var ErrSetPropertyFailed = errors.New("failed to set property on volume")
var ErrSetTTLFailed = errors.New("failed to set ttl on volume")
var ErrStreamInFailed = errors.New("failed to stream in to volume")
var ErrStreamOutFailed = errors.New("failed to stream out from volume")
var ErrStreamOutNotFound = errors.New("no such file or directory")

type VolumeServer struct {
	strategerizer volume.Strategerizer
	volumeRepo    volume.Repository

	logger lager.Logger
}

func NewVolumeServer(
	logger lager.Logger,
	strategerizer volume.Strategerizer,
	volumeRepo volume.Repository,
) *VolumeServer {
	return &VolumeServer{
		strategerizer: strategerizer,
		volumeRepo:    volumeRepo,
		logger:        logger,
	}
}

func (vs *VolumeServer) CreateVolume(w http.ResponseWriter, req *http.Request) {
	var request baggageclaim.VolumeRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	req.Body.Close()
	if err != nil {
		RespondWithError(w, ErrCreateVolumeFailed, http.StatusBadRequest)
		return
	}

	strategy, err := vs.strategerizer.StrategyFor(request)
	if err != nil {
		vs.logger.Error("could-not-produce-strategy", err)
		RespondWithError(w, ErrCreateVolumeFailed, httpUnprocessableEntity)
		return
	}

	createdVolume, err := vs.volumeRepo.CreateVolume(
		request.Handle,
		strategy,
		volume.Properties(request.Properties),
		request.TTLInSeconds,
	)

	if err != nil {
		var code int
		switch err {
		case volume.ErrParentVolumeNotFound:
			code = httpUnprocessableEntity
		case volume.ErrNoParentVolumeProvided:
			code = httpUnprocessableEntity
		default:
			code = http.StatusInternalServerError
		}
		RespondWithError(w, ErrCreateVolumeFailed, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(createdVolume); err != nil {
		vs.logger.Error("failed-to-encode", err, lager.Data{
			"volume-path": createdVolume.Path,
		})
	}
}

func (vs *VolumeServer) ListVolumes(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	properties, err := ConvertQueryToProperties(req.URL.Query())
	if err != nil {
		RespondWithError(w, err, httpUnprocessableEntity)
		return
	}

	volumes, err := vs.volumeRepo.ListVolumes(properties)
	if err != nil {
		RespondWithError(w, ErrListVolumesFailed, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(volumes); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) GetVolume(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	handle := rata.Param(req, "handle")

	vol, found, err := vs.volumeRepo.GetVolume(handle)
	if err != nil {
		RespondWithError(w, ErrGetVolumeFailed, http.StatusInternalServerError)
		return
	}

	if !found {
		RespondWithError(w, ErrGetVolumeFailed, http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(vol); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) GetVolumeStats(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	handle := rata.Param(req, "handle")

	vol, found, err := vs.volumeRepo.GetVolumeStats(handle)
	if err != nil {
		RespondWithError(w, ErrGetVolumeStatsFailed, http.StatusInternalServerError)
		return
	}

	if !found {
		RespondWithError(w, ErrGetVolumeStatsFailed, http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(vol); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) SetProperty(w http.ResponseWriter, req *http.Request) {
	var request baggageclaim.PropertyRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetPropertyFailed, http.StatusBadRequest)
		return
	}

	propertyValue := request.Value
	handle := rata.Param(req, "handle")
	propertyName := rata.Param(req, "property")
	req.Body.Close()

	err = vs.volumeRepo.SetProperty(handle, propertyName, propertyValue)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrSetPropertyFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrSetPropertyFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) SetTTL(w http.ResponseWriter, req *http.Request) {
	var request baggageclaim.TTLRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetTTLFailed, http.StatusBadRequest)
		return
	}

	ttl := request.Value
	handle := rata.Param(req, "handle")
	req.Body.Close()

	err = vs.volumeRepo.SetTTL(handle, ttl)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrSetTTLFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrSetTTLFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) StreamIn(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	handle := rata.Param(req, "handle") // handle of destination volume

	vol, found, err := vs.volumeRepo.GetVolume(handle)
	if err != nil {
		vs.logger.Error("failed-to-get-volume", err, lager.Data{
			"destination-volume-handle": handle,
		})
		RespondWithError(w, ErrStreamInFailed, http.StatusInternalServerError)
		return
	}

	if !found {
		vs.logger.Error("volume-not-found", err, lager.Data{
			"destination-volume-handle": handle,
		})
		RespondWithError(w, ErrStreamInFailed, http.StatusNotFound)
		return
	}

	queryParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		vs.logger.Error("failed-to-parse-query-params", err, lager.Data{
			"destination-volume-handle": handle,
			"raw-query":                 req.URL.RawQuery,
		})
		RespondWithError(w, ErrStreamInFailed, http.StatusInternalServerError)
		return
	}

	var subPath string
	if queryPath, ok := queryParams["path"]; ok {
		subPath = queryPath[0]
	}

	destinationPath := filepath.Join(vol.Path, subPath)
	err = os.MkdirAll(destinationPath, 0755)
	if err != nil {
		vs.logger.Error("failed-to-create-destination-path", err, lager.Data{
			"destination-volume-handle": handle,
			"destination-path":          destinationPath,
		})
		RespondWithError(w, ErrStreamInFailed, http.StatusInternalServerError)
		return
	}

	tarCommand := exec.Command("tar", "-x", "-C", destinationPath)
	tarCommand.Stdin = req.Body

	err = tarCommand.Run()
	if err != nil {
		vs.logger.Error("failed-stream-into-volume", err, lager.Data{
			"destination-volume-handle": handle,
			"destination-path":          destinationPath,
		})

		if _, ok := err.(*exec.ExitError); ok {
			RespondWithError(w, ErrStreamInFailed, http.StatusBadRequest)
			return
		}

		RespondWithError(w, ErrStreamInFailed, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) DestroyVolume(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")
	err := vs.volumeRepo.DestroyVolume(handle)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrSetTTLFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrSetTTLFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) StreamOut(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle") // handle of src volume

	vol, found, err := vs.volumeRepo.GetVolume(handle)
	if err != nil {
		vs.logger.Error("failed-to-get-volume", err, lager.Data{
			"destination-volume-handle": handle,
		})
		RespondWithError(w, ErrStreamOutFailed, http.StatusInternalServerError)
		return
	}

	if !found {
		vs.logger.Error("volume-not-found", err, lager.Data{
			"src-volume-handle": handle,
		})
		RespondWithError(w, ErrStreamOutFailed, http.StatusNotFound)
		return
	}

	queryParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		vs.logger.Error("failed-to-parse-query-params", err, lager.Data{
			"src-volume-handle": handle,
			"raw-query":         req.URL.RawQuery,
		})
		RespondWithError(w, ErrStreamOutFailed, http.StatusInternalServerError)
		return
	}

	var subPath string
	if queryPath, ok := queryParams["path"]; ok {
		subPath = queryPath[0]
	}

	srcPath := filepath.Join(vol.Path, subPath)

	err = vs.streamOutSrc(srcPath, w)
	if err != nil {
		if os.IsNotExist(err) {
			vs.logger.Error("artifact-source-not-found", err, lager.Data{
				"src-volume-handle": handle,
				"src-path":          srcPath,
			})
			RespondWithError(w, ErrStreamOutNotFound, http.StatusNotFound)
			return
		}

		vs.logger.Error("failed-stream-out-from-volume", err, lager.Data{
			"src-volume-handle": handle,
			"src-path":          srcPath,
		})

		if _, ok := err.(*exec.ExitError); ok {
			RespondWithError(w, ErrStreamOutFailed, http.StatusBadRequest)
			return
		}

		RespondWithError(w, ErrStreamOutFailed, http.StatusInternalServerError)
		return
	}
}

func (vs *VolumeServer) streamOutSrc(srcPath string, w http.ResponseWriter) error {
	fileInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	var tarCommandPath, tarCommandDir string

	if fileInfo.IsDir() {
		tarCommandPath = "."
		tarCommandDir = srcPath
	} else {
		tarCommandPath = filepath.Base(srcPath)
		tarCommandDir = filepath.Dir(srcPath)
	}

	tarCommand := exec.Command("tar", "-c", tarCommandPath)
	tarCommand.Dir = tarCommandDir

	readCloser, err := tarCommand.StdoutPipe()
	if err != nil {
		return err
	}

	err = tarCommand.Start()
	if err != nil {
		return err
	}

	_, err = io.Copy(w, readCloser)
	if err != nil {
		return err
	}

	return tarCommand.Wait()
}
