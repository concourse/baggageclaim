package api

import (
	"encoding/json"
	"net/http"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/baggageclaim"
)

type Info struct {
	ProtocolVersion int `json:"protocol_version"`
}

type infoServer struct {
	logger lager.Logger
}

func NewInfoServer(
	logger lager.Logger,
) *infoServer {
	return &infoServer{
		logger: logger,
	}
}

func (is *infoServer) Info(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(Info{
		ProtocolVersion: baggageclaim.ProtocolVersion,
	})

	if err != nil {
		is.logger.Error("failed-to-encode", err)
		RespondWithError(w, ErrGetVolumeFailed, http.StatusInternalServerError)
	}
}
