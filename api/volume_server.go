package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/tedsuo/rata"
)

const httpUnprocessableEntity = 422

var ErrListVolumesFailed = errors.New("failed to list volumes")
var ErrGetVolumeFailed = errors.New("failed to get volume")
var ErrCreateVolumeFailed = errors.New("failed to create volume")
var ErrDestroyVolumeFailed = errors.New("failed to destroy volume")
var ErrSetPropertyFailed = errors.New("failed to set property on volume")
var ErrSetTTLFailed = errors.New("failed to set ttl on volume")
var ErrSetPrivilegedFailed = errors.New("failed to change privileged status of volume")
var ErrStreamInFailed = errors.New("failed to stream in to volume")
var ErrStreamOutFailed = errors.New("failed to stream out from volume")
var ErrStreamOutNotFound = errors.New("no such file or directory")

type VolumeServer struct {
	strategerizer  volume.Strategerizer
	volumeRepo     volume.Repository
	volumePromises volume.PromiseList

	logger lager.Logger
}

func NewVolumeServer(
	logger lager.Logger,
	strategerizer volume.Strategerizer,
	volumeRepo volume.Repository,
) *VolumeServer {
	return &VolumeServer{
		strategerizer:  strategerizer,
		volumeRepo:     volumeRepo,
		volumePromises: volume.NewPromiseList(),
		logger:         logger,
	}
}

func (vs *VolumeServer) CreateVolume(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("create-volume")

	hLog.Debug("start")
	defer hLog.Debug("done")

	request, handle, strategy, hLog, err := vs.prepareCreate(w, req, hLog)
	if err != nil {
		return
	}

	createdVolume, err := vs.doCreate(w, request, handle, strategy, hLog, vs)
	if err != nil {
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(createdVolume); err != nil {
		hLog.Error("failed-to-encode", err, lager.Data{
			"volume-path": createdVolume.Path,
		})
	}
}

func (vs *VolumeServer) CreateVolumeAsync(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("create-volume-async")

	hLog.Debug("start")
	defer hLog.Debug("done")

	request, handle, strategy, hLog, err := vs.prepareCreate(w, req, hLog)
	if err != nil {
		return
	}

	handlers := &createVolumeHandlerAsync{
		promise: volume.NewPromise(),
		server:  vs,
	}

	err = vs.volumePromises.AddPromise(handle, handlers.promise)
	if err != nil {
		hLog.Error("failed-to-add-promise", err)
		RespondWithError(w, err, http.StatusInternalServerError)
		return
	}

	go vs.doCreate(w, request, handle, strategy, hLog, handlers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	future := baggageclaim.VolumeFutureResponse{
		Handle: handle,
	}

	if err := json.NewEncoder(w).Encode(future); err != nil {
		hLog.Error("failed-to-encode", err, lager.Data{
			"future": future.Handle,
		})
	}
}

func (vs *VolumeServer) CreateVolumeAsyncCancel(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("create-volume-async-cancel", lager.Data{
		"handle": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	promise := vs.volumePromises.GetPromise(handle)
	if promise == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if promise.IsPending() {
		err := promise.Reject(volume.ErrPromiseCanceled)
		if err != nil {
			hLog.Error("failed-to-reject-promise", err)
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}
	}

	vs.volumePromises.RemovePromise(handle)

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) CreateVolumeAsyncCheck(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("create-volume-async-check", lager.Data{
		"handle": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	promise := vs.volumePromises.GetPromise(handle)
	if promise == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if promise.IsPending() {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	createdVolume, err, errPromise := promise.GetValue()
	if errPromise != nil {
		hLog.Error("failed-to-get-value-from-promise", errPromise)
		RespondWithError(w, errPromise, http.StatusInternalServerError)
		return
	}

	if err != nil {
		vs.creationFailed(w, err)
		return
	}

	hLog = hLog.WithData(lager.Data{
		"volume": createdVolume.Handle,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(createdVolume); err != nil {
		hLog.Error("failed-to-encode", err, lager.Data{
			"volume-path": createdVolume.Path,
		})
	}
}

func (vs *VolumeServer) DestroyVolume(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("destroy", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	err := vs.volumeRepo.DestroyVolume(handle)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			hLog.Info("volume-does-not-exist")
			RespondWithError(w, ErrDestroyVolumeFailed, http.StatusNotFound)
		} else {
			hLog.Error("failed-to-destroy", err)
			RespondWithError(w, ErrDestroyVolumeFailed, http.StatusInternalServerError)
		}

		return
	}

	hLog.Info("destroyed")

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) ListVolumes(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("list-volumes")

	hLog.Debug("start")
	defer hLog.Debug("done")

	w.Header().Set("Content-Type", "application/json")

	properties, err := ConvertQueryToProperties(req.URL.Query())
	if err != nil {
		RespondWithError(w, err, httpUnprocessableEntity)
		return
	}

	volumes, _, err := vs.volumeRepo.ListVolumes(properties)
	if err != nil {
		hLog.Error("failed-to-list-volumes", err)
		RespondWithError(w, ErrListVolumesFailed, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(volumes); err != nil {
		hLog.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) GetVolume(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("get-volume", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	vol, found, err := vs.volumeRepo.GetVolume(handle)
	if err != nil {
		hLog.Error("failed-to-get-volume", err)
		RespondWithError(w, ErrGetVolumeFailed, http.StatusInternalServerError)
		return
	}

	if !found {
		hLog.Info("volume-not-found")
		RespondWithError(w, ErrGetVolumeFailed, http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(vol); err != nil {
		hLog.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) SetProperty(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")
	propertyName := rata.Param(req, "property")

	hLog := vs.logger.Session("set-property", lager.Data{
		"volume":   handle,
		"property": propertyName,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	var request baggageclaim.PropertyRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetPropertyFailed, http.StatusBadRequest)
		return
	}

	propertyValue := request.Value

	hLog.Debug("setting-property")

	err = vs.volumeRepo.SetProperty(handle, propertyName, propertyValue)
	if err != nil {
		hLog.Error("failed-to-set-property", err)

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
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("set-ttl", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	var request baggageclaim.TTLRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetTTLFailed, http.StatusBadRequest)
		return
	}

	ttl := request.Value

	hLog.Debug("setting-ttl", lager.Data{"ttl": ttl})

	err = vs.volumeRepo.SetTTL(handle, ttl)
	if err != nil {
		hLog.Error("failed-to-set-ttl", err)

		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrSetTTLFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrSetTTLFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) SetPrivileged(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("set-privileged", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	var request baggageclaim.PrivilegedRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetPrivilegedFailed, http.StatusBadRequest)
		return
	}

	privileged := request.Value

	hLog.Debug("setting-privileged", lager.Data{"privileged": privileged})

	err = vs.volumeRepo.SetPrivileged(handle, privileged)
	if err != nil {
		hLog.Error("failed-to-change-privileged-status", err)

		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrSetPrivilegedFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrSetPrivilegedFailed, http.StatusInternalServerError)
		}

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) StreamIn(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("stream-in", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	var subPath string
	if queryPath, ok := req.URL.Query()["path"]; ok {
		subPath = queryPath[0]
	}

	badStream, err := vs.volumeRepo.StreamIn(handle, subPath, req.Body)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			hLog.Info("volume-not-found")
			RespondWithError(w, ErrStreamInFailed, http.StatusNotFound)
			return
		}

		if badStream {
			hLog.Info("bad-stream-payload", lager.Data{"error": err.Error()})
			RespondWithError(w, ErrStreamInFailed, http.StatusBadRequest)
			return
		}

		hLog.Error("failed-to-stream-into-volume", err)
		RespondWithError(w, ErrStreamInFailed, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) StreamOut(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("stream-out", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	var subPath string
	if queryPath, ok := req.URL.Query()["path"]; ok {
		subPath = queryPath[0]
	}

	err := vs.volumeRepo.StreamOut(handle, subPath, w)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			hLog.Info("volume-not-found")
			RespondWithError(w, ErrStreamOutFailed, http.StatusNotFound)
			return
		}

		if os.IsNotExist(err) {
			hLog.Info("source-path-not-found")
			RespondWithError(w, ErrStreamOutNotFound, http.StatusNotFound)
			return
		}

		hLog.Error("failed-to-stream-out", err)
		RespondWithError(w, ErrStreamOutFailed, http.StatusInternalServerError)
		return
	}
}

func (vs *VolumeServer) generateHandle() (string, error) {
	handle, err := uuid.NewV4()
	if err != nil {
		return "", err
	}

	return handle.String(), nil
}

func (vs *VolumeServer) prepareCreate(w http.ResponseWriter, req *http.Request, hLog lager.Logger) (baggageclaim.VolumeRequest, string, volume.Strategy, lager.Logger, error) {
	var request baggageclaim.VolumeRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		hLog.Error("failed-to-decode-request", err)
		RespondWithError(w, ErrCreateVolumeFailed, http.StatusBadRequest)
		return baggageclaim.VolumeRequest{}, "", nil, hLog, err
	}

	handle := request.Handle
	if handle == "" {
		handle, err = vs.generateHandle()
		if err != nil {
			hLog.Error("failed-to-generate-handle", err)
			RespondWithError(w, ErrCreateVolumeFailed, http.StatusBadRequest)
			return baggageclaim.VolumeRequest{}, "", nil, hLog, err
		}
	}

	hLog = hLog.WithData(lager.Data{
		"handle":     handle,
		"ttl":        request.TTLInSeconds,
		"privileged": request.Privileged,
		"strategy":   request.Strategy,
	})

	strategy, err := vs.strategerizer.StrategyFor(request)
	if err != nil {
		hLog.Error("could-not-produce-strategy", err)
		RespondWithError(w, ErrCreateVolumeFailed, httpUnprocessableEntity)
		return baggageclaim.VolumeRequest{}, "", nil, hLog, err
	}

	return request, handle, strategy, hLog, nil
}

func (vs *VolumeServer) doCreate(w http.ResponseWriter, request baggageclaim.VolumeRequest, handle string, strategy volume.Strategy, hLog lager.Logger, handlers volumeCreationHandler) (volume.Volume, error) {
	hLog.Debug("creating")

	createdVolume, err := vs.volumeRepo.CreateVolume(
		handle,
		strategy,
		volume.Properties(request.Properties),
		request.TTLInSeconds,
		request.Privileged,
	)

	if err != nil {
		hLog.Error("failed-to-create", err)

		return handlers.creationFailed(w, err)
	}

	hLog = hLog.WithData(lager.Data{
		"volume": createdVolume.Handle,
	})

	hLog.Debug("created")

	return handlers.creationSucceeded(createdVolume, hLog)
}

type volumeCreationHandler interface {
	creationFailed(w http.ResponseWriter, err error) (volume.Volume, error)
	creationSucceeded(createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error)
}

func (vs *VolumeServer) creationFailed(w http.ResponseWriter, err error) (volume.Volume, error) {
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
	return volume.Volume{}, err
}

func (vs *VolumeServer) creationSucceeded(createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error) {
	return createdVolume, nil
}

type createVolumeHandlerAsync struct {
	promise volume.Promise
	server  *VolumeServer
}

func (h *createVolumeHandlerAsync) creationFailed(w http.ResponseWriter, err error) (volume.Volume, error) {
	h.promise.Reject(err)

	return volume.Volume{}, err
}

func (h *createVolumeHandlerAsync) creationSucceeded(createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error) {
	err := h.promise.Fulfill(createdVolume)
	if err != nil {
		if err == volume.ErrPromiseCanceled {
			hLog.Info("promise-was-canceled")

			hLogDestroy := h.server.logger.Session("destroy", lager.Data{
				"volume": createdVolume.Handle,
			})

			hLogDestroy.Debug("start")
			defer hLogDestroy.Debug("done")

			err := h.server.volumeRepo.DestroyVolume(createdVolume.Handle)
			if err != nil {
				if err != volume.ErrVolumeDoesNotExist {
					hLog.Error("failed-to-destroy", err)
				}
			}
		} else {
			hLog.Error("failed-to-fulfill-promise", err)
		}

		return volume.Volume{}, err
	}

	return createdVolume, nil
}
