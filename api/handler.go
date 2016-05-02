package api

import (
	"encoding/json"
	"net/http"

	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
)

func NewHandler(
	logger lager.Logger,
	strategerizer volume.Strategerizer,
	volumeRepo volume.Repository,
) (http.Handler, error) {
	volumeServer := NewVolumeServer(
		logger.Session("volume-server"),
		strategerizer,
		volumeRepo,
	)

	handlers := rata.Handlers{
		baggageclaim.CreateVolume: http.HandlerFunc(volumeServer.CreateVolume),
		baggageclaim.ListVolumes:  http.HandlerFunc(volumeServer.ListVolumes),
		baggageclaim.GetVolume:    http.HandlerFunc(volumeServer.GetVolume),
		baggageclaim.SetProperty:  http.HandlerFunc(volumeServer.SetProperty),
		baggageclaim.SetTTL:       http.HandlerFunc(volumeServer.SetTTL),
		baggageclaim.StreamIn:     http.HandlerFunc(volumeServer.StreamIn),
	}

	return rata.NewRouter(baggageclaim.Routes, handlers)
}

type errorResponse struct {
	Message string `json:"error"`
}

func RespondWithError(w http.ResponseWriter, err error, statusCode ...int) {
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
