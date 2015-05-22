package api

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/mattermaster"
	"github.com/concourse/mattermaster/volume"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

func NewHandler(logger lager.Logger, volumeDir string) (http.Handler, error) {
	volumeRepo := volume.NewRepository(
		logger.Session("repository"),
		volumeDir,
	)

	volumeServer := NewVolumeServer(
		logger.Session("volume-server"),
		volumeRepo,
	)

	handlers := rata.Handlers{
		mattermaster.CreateVolume: http.HandlerFunc(volumeServer.CreateVolume),
		mattermaster.GetVolumes:   http.HandlerFunc(volumeServer.GetVolumes),
	}

	return rata.NewRouter(mattermaster.Routes, handlers)
}

type errorResponse struct {
	Message string `json:"error"`
}

func respondWithError(w http.ResponseWriter, err error, statusCode ...int) {
	var code int

	if len(statusCode) > 0 {
		code = statusCode[0]
	} else {
		code = http.StatusInternalServerError
	}

	w.WriteHeader(code)
	errResponse := errorResponse{Message: err.Error()}
	json.NewEncoder(w).Encode(errResponse)
}
