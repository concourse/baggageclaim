package api

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	"github.com/tedsuo/rata"
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
		baggageclaim.CreateVolume:            http.HandlerFunc(volumeServer.CreateVolume),
		baggageclaim.CreateVolumeAsync:       http.HandlerFunc(volumeServer.CreateVolumeAsync),
		baggageclaim.CreateVolumeAsyncCancel: http.HandlerFunc(volumeServer.CreateVolumeAsyncCancel),
		baggageclaim.CreateVolumeAsyncCheck:  http.HandlerFunc(volumeServer.CreateVolumeAsyncCheck),
		baggageclaim.ListVolumes:             http.HandlerFunc(volumeServer.ListVolumes),
		baggageclaim.GetVolume:               http.HandlerFunc(volumeServer.GetVolume),
		baggageclaim.SetProperty:             http.HandlerFunc(volumeServer.SetProperty),
		baggageclaim.GetPrivileged:           http.HandlerFunc(volumeServer.GetPrivileged),
		baggageclaim.SetPrivileged:           http.HandlerFunc(volumeServer.SetPrivileged),
		baggageclaim.StreamIn:                http.HandlerFunc(volumeServer.StreamIn),
		baggageclaim.StreamOut:               http.HandlerFunc(volumeServer.StreamOut),
		baggageclaim.DestroyVolume:           http.HandlerFunc(volumeServer.DestroyVolume),
		baggageclaim.DestroyVolumes:          http.HandlerFunc(volumeServer.DestroyVolumes),

		baggageclaim.GetBase:      http.HandlerFunc(volumeServer.GetBase),
		baggageclaim.GetManifest:  http.HandlerFunc(volumeServer.VolumeImageHandlerFor(volumeServer.GetManifest)),
		baggageclaim.HeadManifest: http.HandlerFunc(volumeServer.VolumeImageHandlerFor(volumeServer.GetManifest)),
		baggageclaim.GetBlob:      http.HandlerFunc(volumeServer.VolumeImageHandlerFor(volumeServer.GetBlob)),
		baggageclaim.HeadBlob:     http.HandlerFunc(volumeServer.VolumeImageHandlerFor(volumeServer.GetBlob)),
	}

	return rata.NewRouter(baggageclaim.Routes, handlers)
}

type ErrorResponse struct {
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
	errResponse := ErrorResponse{Message: err.Error()}
	json.NewEncoder(w).Encode(errResponse)
}
