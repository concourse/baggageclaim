package api

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

const httpUnprocessableEntity = 422

type VolumeRequest struct {
	Strategy     volume.Strategy   `json:"strategy"`
	Properties   volume.Properties `json:"properties"`
	TTLInSeconds uint              `json:"ttl,omitempty"`
}

type PropertyRequest struct {
	Value string `json:"value"`
}

type TTLRequest struct {
	Value uint `json:"value"`
}

type VolumeServer struct {
	volumeRepo volume.Repository

	logger lager.Logger
}

func NewVolumeServer(logger lager.Logger, volumeRepo volume.Repository) *VolumeServer {
	return &VolumeServer{
		volumeRepo: volumeRepo,
		logger:     logger,
	}
}

func (vs *VolumeServer) CreateVolume(w http.ResponseWriter, req *http.Request) {
	var request VolumeRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	req.Body.Close()
	if err != nil {
		respondWithError(w, volume.ErrCreateVolumeFailed, http.StatusBadRequest)
		return
	}

	createdVolume, err := vs.volumeRepo.CreateVolume(
		request.Strategy,
		request.Properties,
		request.TTLInSeconds,
	)

	if err != nil {
		var code int
		switch err {
		case volume.ErrParentVolumeNotFound:
			code = httpUnprocessableEntity
		case volume.ErrNoParentVolumeProvided:
			code = httpUnprocessableEntity
		case volume.ErrMissingStrategy:
			code = httpUnprocessableEntity
		case volume.ErrUnrecognizedStrategy:
			code = httpUnprocessableEntity
		default:
			code = http.StatusInternalServerError
		}
		respondWithError(w, volume.ErrCreateVolumeFailed, code)
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

func (vs *VolumeServer) GetVolumes(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	properties, err := ConvertQueryToProperties(req.URL.Query())
	if err != nil {
		respondWithError(w, err, httpUnprocessableEntity)
		return
	}

	volumes, err := vs.volumeRepo.ListVolumes(properties)
	if err != nil {
		respondWithError(w, volume.ErrListVolumesFailed, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(volumes); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) SetProperty(w http.ResponseWriter, req *http.Request) {
	var request PropertyRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		respondWithError(w, volume.ErrSetPropertyFailed, http.StatusBadRequest)
		return
	}

	propertyValue := request.Value
	handle := rata.Param(req, "handle")
	propertyName := rata.Param(req, "property")
	req.Body.Close()

	err = vs.volumeRepo.SetProperty(handle, propertyName, propertyValue)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			respondWithError(w, volume.ErrSetPropertyFailed, http.StatusNotFound)
		} else {
			respondWithError(w, volume.ErrSetPropertyFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) SetTTL(w http.ResponseWriter, req *http.Request) {
	var request TTLRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		respondWithError(w, volume.ErrSetTTLFailed, http.StatusBadRequest)
		return
	}

	ttl := request.Value
	handle := rata.Param(req, "handle")
	req.Body.Close()

	err = vs.volumeRepo.SetTTL(handle, ttl)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			respondWithError(w, volume.ErrSetTTLFailed, http.StatusNotFound)
		} else {
			respondWithError(w, volume.ErrSetTTLFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
