package api

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

const httpUnprocessableEntity = 422

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
	var request baggageclaim.VolumeRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	req.Body.Close()
	if err != nil {
		RespondWithError(w, volume.ErrCreateVolumeFailed, http.StatusBadRequest)
		return
	}

	if request.Strategy == nil {
		RespondWithError(w, volume.ErrCreateVolumeFailed, httpUnprocessableEntity)
		return
	}

	var strategy volume.Strategy
	err = json.Unmarshal(*request.Strategy, &strategy)
	if err != nil {
		RespondWithError(w, volume.ErrCreateVolumeFailed, http.StatusBadRequest)
		return
	}

	createdVolume, err := vs.volumeRepo.CreateVolume(
		strategy,
		volume.Properties(request.Properties),
		request.TTLInSeconds,
		request.Privileged,
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
		RespondWithError(w, volume.ErrCreateVolumeFailed, code)
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
		RespondWithError(w, volume.ErrListVolumesFailed, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(volumes); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) GetVolume(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	handle := rata.Param(req, "handle")

	vol, err := vs.volumeRepo.GetVolume(handle)
	if err != nil {
		RespondWithError(w, volume.ErrGetVolumeFailed, http.StatusInternalServerError)
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
		RespondWithError(w, volume.ErrSetPropertyFailed, http.StatusBadRequest)
		return
	}

	propertyValue := request.Value
	handle := rata.Param(req, "handle")
	propertyName := rata.Param(req, "property")
	req.Body.Close()

	err = vs.volumeRepo.SetProperty(handle, propertyName, propertyValue)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, volume.ErrSetPropertyFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, volume.ErrSetPropertyFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) SetTTL(w http.ResponseWriter, req *http.Request) {
	var request baggageclaim.TTLRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, volume.ErrSetTTLFailed, http.StatusBadRequest)
		return
	}

	ttl := request.Value
	handle := rata.Param(req, "handle")
	req.Body.Close()

	err = vs.volumeRepo.SetTTL(handle, ttl)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, volume.ErrSetTTLFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, volume.ErrSetTTLFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
