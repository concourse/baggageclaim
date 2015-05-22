package api

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/mattermaster/volume"
	"github.com/pivotal-golang/lager"
)

type VolumeRequest struct {
	Strategy volume.Strategy `json:"strategy"`
}

type VolumeServer struct {
	volumeRepo *volume.Repository

	logger lager.Logger
}

func NewVolumeServer(logger lager.Logger, volumeRepo *volume.Repository) *VolumeServer {
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

	createdVolume, err := vs.volumeRepo.CreateVolume(request.Strategy)
	if err != nil {
		var code int
		switch err {
		case volume.ErrParentVolumeNotFound:
			code = 422
		case volume.ErrNoParentVolumeProvided:
			code = 422
		case volume.ErrMissingStrategy:
			code = 422
		case volume.ErrUnrecognizedStrategy:
			code = 422
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

	volumes, err := vs.volumeRepo.ListVolumes()
	if err != nil {
		respondWithError(w, volume.ErrListVolumesFailed, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(volumes); err != nil {
		vs.logger.Error("failed-to-encode", err)
	}
}
