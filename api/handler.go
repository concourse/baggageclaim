package api

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/mattermaster"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

func NewHandler(logger lager.Logger, volumeDir string) (http.Handler, error) {
	volumeServer := NewVolumeServer(
		logger.Session("volume-server"),
		volumeDir,
	)

	handlers := rata.Handlers{
		mattermaster.CreateVolume: http.HandlerFunc(volumeServer.CreateVolume),
	}

	return rata.NewRouter(mattermaster.Routes, handlers)
}

type errorResponse struct {
	Message string `json:"error"`
}

func respondWithError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	errResponse := errorResponse{Message: err.Error()}
	json.NewEncoder(w).Encode(errResponse)
}
