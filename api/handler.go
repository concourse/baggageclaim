package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/concourse/mattermaster"
	"github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"
)

func NewHandler(volumeDir string) (http.Handler, error) {
	volumeServer := volumeServer{
		volumeDir: volumeDir,
	}

	handlers := rata.Handlers{
		mattermaster.CreateVolume: http.HandlerFunc(volumeServer.createHandler),
	}

	return rata.NewRouter(mattermaster.Routes, handlers)
}

type CreateVolumeResponse struct {
	Path string
}

type volumeServer struct {
	volumeDir string
}

func (vs volumeServer) createHandler(w http.ResponseWriter, req *http.Request) {
	guid, err := uuid.NewV4()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	createdVolume := filepath.Join(vs.volumeDir, guid.String())

	os.MkdirAll(createdVolume, 0755)

	createVolumeResponse := CreateVolumeResponse{
		Path: createdVolume,
	}

	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(createVolumeResponse); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
