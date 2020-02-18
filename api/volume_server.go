package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	uuid "github.com/nu7hatch/gouuid"
	"github.com/prometheus/common/log"
	"github.com/tedsuo/rata"

	"github.com/docker/distribution/registry/api/errcode"
	v2 "github.com/docker/distribution/registry/api/v2"
)

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

const (
	RegistryHeaderKey   = "docker-distribution-api-version"
	RegistryHeaderValue = "registry/2.0"

	httpUnprocessableEntity = 422
)

var (
	handleRegexp = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
)

func (vs *VolumeServer) GetBase(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(RegistryHeaderKey, RegistryHeaderValue)

	return
}

func (vs *VolumeServer) GetBlob(img volume.Image) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc, size, err := img.GetBlob(context.Background(), rata.Param(r, "ref"))
		if err != nil {
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}

		defer rc.Close()

		w.Header().Set("content-length", strconv.Itoa(int(size)))

		if strings.ToLower(r.Method) == "head" {
			return
		}

		_, err = io.Copy(w, rc)
		if err != nil {
			vs.logger.Error("copy-blob", err)
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}
	})
}

func (vs *VolumeServer) GetManifest(img volume.Image) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, desc, err := img.GetManifest(context.Background())
		if err != nil {
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}

		w.Header().Set("content-type", desc.MediaType)
		w.Header().Set("content-length", strconv.Itoa(int(desc.Size)))
		w.Header().Set("docker-content-digest", desc.Digest.String())

		if strings.ToLower(r.Method) == "head" {
			return
		}

		_, err = w.Write(b)
		if err != nil {
			vs.logger.Error("writing-manifest", err)
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}

		return
	})
}

func (vs *VolumeServer) VolumeImageHandlerFor(
	handler func(volume.Image) http.Handler,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			ctx    = context.Background()
			errs   = errcode.Errors{}
			handle = rata.Param(r, "handle")
		)

		w.Header().Set(RegistryHeaderKey, RegistryHeaderValue)

		if !handleRegexp.MatchString(handle) {
			errs = append(errs, v2.ErrorCodeNameInvalid.WithDetail(
				"only `latest` is supported",
			))

			errcode.ServeJSON(w, errs)
			return
		}

		vol, found, err := vs.volumeRepo.GetVolume(ctx, handle)
		if err != nil {
			log.Error("get-volume", err)
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}

		if !found {
			errs = append(errs, v2.ErrorCodeManifestUnknown.WithDetail(
				"couldn't find volume",
			))

			log.Info("volume-not-found")
			errcode.ServeJSON(w, errs)
			return
		}

		img, err := volume.NewImage(ctx, vol)
		if err != nil {
			log.Error("new-image", err)
			RespondWithError(w, err, http.StatusInternalServerError)
			return
		}

		handler(img).ServeHTTP(w, r)
	}
}

func (vs *VolumeServer) CreateVolume(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("create-volume")

	hLog.Debug("start")
	defer hLog.Debug("done")

	ctx := lagerctx.NewContext(req.Context(), hLog)

	request, handle, strategy, hLog, err := vs.prepareCreate(w, req, hLog)
	if err != nil {
		return
	}

	createdVolume, err := vs.doCreate(ctx, w, request, handle, strategy, hLog, vs)
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

	ctx := lagerctx.NewContext(req.Context(), hLog)

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

	go vs.doCreate(ctx, w, request, handle, strategy, hLog, handlers)

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

	ctx := lagerctx.NewContext(req.Context(), hLog)

	err := vs.volumeRepo.DestroyVolume(ctx, handle)
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

func (vs *VolumeServer) DestroyVolumes(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("destroy-volumes")

	hLog.Debug("start")
	defer hLog.Debug("done")

	w.Header().Set("Content-Type", "application/json")
	var volumes []string
	err := json.NewDecoder(req.Body).Decode(&volumes)

	if err != nil {
		hLog.Error("failed-to-destroy-volumes", err)
		RespondWithError(w, ErrDestroyVolumeFailed, http.StatusBadRequest)
		return
	}

	var handleWg sync.WaitGroup
	var errChan = make(chan error, len(volumes))

	for _, handle := range volumes {
		volumeLog := hLog.Session("destroy", lager.Data{"handle": handle})

		ctx := lagerctx.NewContext(req.Context(), volumeLog)

		handleWg.Add(1)
		go func(handle string) {
			defer handleWg.Done()

			err := vs.volumeRepo.DestroyVolume(ctx, handle)
			if err != nil {
				if err == volume.ErrVolumeDoesNotExist {
					volumeLog.Info("volume-does-not-exist")
				} else {
					volumeLog.Error("failed-to-destroy-volume", err)
					errChan <- err
				}
			}
		}(handle)
	}

	hLog.Debug("waiting-for-volumes-to-be-destroyed")

	handleWg.Wait()
	close(errChan)

	if len(errChan) > 0 {
		RespondWithError(w, ErrDestroyVolumeFailed, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (vs *VolumeServer) ListVolumes(w http.ResponseWriter, req *http.Request) {
	hLog := vs.logger.Session("list-volumes")

	hLog.Debug("start")
	defer hLog.Debug("done")

	ctx := lagerctx.NewContext(req.Context(), hLog)

	w.Header().Set("Content-Type", "application/json")

	properties, err := ConvertQueryToProperties(req.URL.Query())
	if err != nil {
		RespondWithError(w, err, httpUnprocessableEntity)
		return
	}

	volumes, _, err := vs.volumeRepo.ListVolumes(ctx, properties)
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

	ctx := lagerctx.NewContext(req.Context(), hLog)

	vol, found, err := vs.volumeRepo.GetVolume(ctx, handle)
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

	ctx := lagerctx.NewContext(req.Context(), hLog)

	var request baggageclaim.PropertyRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetPropertyFailed, http.StatusBadRequest)
		return
	}

	propertyValue := request.Value

	hLog.Debug("setting-property")

	err = vs.volumeRepo.SetProperty(ctx, handle, propertyName, propertyValue)
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

func (vs *VolumeServer) GetPrivileged(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("get-privileged", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	ctx := lagerctx.NewContext(req.Context(), hLog)

	privileged, err := vs.volumeRepo.GetPrivileged(ctx, handle)
	if err != nil {
		hLog.Error("failed-to-change-privileged-status", err)

		if err == volume.ErrVolumeDoesNotExist {
			RespondWithError(w, ErrGetPrivilegedFailed, http.StatusNotFound)
		} else {
			RespondWithError(w, ErrGetPrivilegedFailed, http.StatusInternalServerError)
		}

		return
	}

	if err := json.NewEncoder(w).Encode(privileged); err != nil {
		hLog.Error("failed-to-encode", err)
	}
}

func (vs *VolumeServer) SetPrivileged(w http.ResponseWriter, req *http.Request) {
	handle := rata.Param(req, "handle")

	hLog := vs.logger.Session("set-privileged", lager.Data{
		"volume": handle,
	})

	hLog.Debug("start")
	defer hLog.Debug("done")

	ctx := lagerctx.NewContext(req.Context(), hLog)

	var request baggageclaim.PrivilegedRequest
	err := json.NewDecoder(req.Body).Decode(&request)
	if err != nil {
		RespondWithError(w, ErrSetPrivilegedFailed, http.StatusBadRequest)
		return
	}

	privileged := request.Value

	hLog.Debug("setting-privileged", lager.Data{"privileged": privileged})

	err = vs.volumeRepo.SetPrivileged(ctx, handle, privileged)
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

	ctx := lagerctx.NewContext(req.Context(), hLog)

	var subPath string
	if queryPath, ok := req.URL.Query()["path"]; ok {
		subPath = queryPath[0]
	}

	badStream, err := vs.volumeRepo.StreamIn(ctx, handle, subPath, req.Header.Get("Content-Encoding"), req.Body)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			hLog.Info("volume-not-found")
			RespondWithError(w, ErrStreamInFailed, http.StatusNotFound)
			return
		}

		if err == volume.ErrUnsupportedStreamEncoding {
			hLog.Info("unsupported-stream-encoding")
			RespondWithError(w, ErrStreamInFailed, http.StatusBadRequest)
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

	ctx := lagerctx.NewContext(req.Context(), hLog)

	var subPath string
	if queryPath, ok := req.URL.Query()["path"]; ok {
		subPath = queryPath[0]
	}

	err := vs.volumeRepo.StreamOut(ctx, handle, subPath, req.Header.Get("Accept-Encoding"), w)
	if err != nil {
		if err == volume.ErrVolumeDoesNotExist {
			hLog.Info("volume-not-found")
			RespondWithError(w, ErrStreamOutFailed, http.StatusNotFound)
			return
		}

		if err == volume.ErrUnsupportedStreamEncoding {
			hLog.Info("unsupported-stream-encoding")
			RespondWithError(w, ErrStreamOutFailed, http.StatusBadRequest)
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

func (vs *VolumeServer) doCreate(ctx context.Context, w http.ResponseWriter, request baggageclaim.VolumeRequest, handle string, strategy volume.Strategy, hLog lager.Logger, handlers volumeCreationHandler) (volume.Volume, error) {
	hLog.Debug("creating")

	createdVolume, err := vs.volumeRepo.CreateVolume(
		ctx,
		handle,
		strategy,
		volume.Properties(request.Properties),
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

	return handlers.creationSucceeded(ctx, createdVolume, hLog)
}

type volumeCreationHandler interface {
	creationFailed(w http.ResponseWriter, err error) (volume.Volume, error)
	creationSucceeded(ctx context.Context, createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error)
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

func (vs *VolumeServer) creationSucceeded(ctx context.Context, createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error) {
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

func (h *createVolumeHandlerAsync) creationSucceeded(ctx context.Context, createdVolume volume.Volume, hLog lager.Logger) (volume.Volume, error) {
	err := h.promise.Fulfill(createdVolume)
	if err != nil {
		if err == volume.ErrPromiseCanceled {
			hLog.Info("promise-was-canceled")

			hLogDestroy := lagerctx.FromContext(ctx).Session("destroy-via-cancel", lager.Data{
				"volume": createdVolume.Handle,
			})

			hLogDestroy.Debug("start")
			defer hLogDestroy.Debug("done")

			err := h.server.volumeRepo.DestroyVolume(lagerctx.NewContext(ctx, hLogDestroy), createdVolume.Handle)
			if err != nil {
				if err != volume.ErrVolumeDoesNotExist {
					hLogDestroy.Error("failed-to-destroy", err)
				}
			}
		} else {
			hLog.Error("failed-to-fulfill-promise", err)
		}

		return volume.Volume{}, err
	}

	return createdVolume, nil
}
