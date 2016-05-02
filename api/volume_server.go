package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

const httpUnprocessableEntity = 422

var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrGetVolumeFailed = errors.New("failed to get volume")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrDestroyVolumeFailed = errors.New("failed to destroy volume")
var ErrSetPropertyFailed = errors.New("failed to set property on volume")
var ErrSetTTLFailed = errors.New("failed to set ttl on volume")
var ErrStreamInFailed = errors.New("failed to stream in to volume")

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
