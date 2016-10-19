package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/rata"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/retryhttp"
)

type Client interface {
	baggageclaim.Client
}

type client struct {
	requestGenerator *rata.RequestGenerator

	retryBackOffFactory retryhttp.BackOffFactory
	nestedRoundTripper  http.RoundTripper
}

func New(apiURL string) Client {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, baggageclaim.Routes),

		retryBackOffFactory: retryhttp.NewExponentialBackOffFactory(60 * time.Minute),

		nestedRoundTripper: &http.Transport{
			DisableKeepAlives: true,
		},
	}
}

func (c *client) httpClient(logger lager.Logger) *http.Client {
	return &http.Client{
		Transport: &retryhttp.RetryRoundTripper{
			Logger:         logger.Session("retry-round-tripper"),
			BackOffFactory: c.retryBackOffFactory,
			RoundTripper:   c.nestedRoundTripper,
		},
	}
}

func (c *client) CreateVolume(logger lager.Logger, handle string, volumeSpec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
	strategy := volumeSpec.Strategy
	if strategy == nil {
		strategy = baggageclaim.EmptyStrategy{}
	}

	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(baggageclaim.VolumeRequest{
		Handle:       handle,
		Strategy:     strategy.Encode(),
		TTLInSeconds: uint(math.Ceil(volumeSpec.TTL.Seconds())),
		Properties:   volumeSpec.Properties,
		Privileged:   volumeSpec.Privileged,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != 201 {
		return nil, getError(response)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return nil, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse baggageclaim.VolumeResponse
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return nil, err
	}

	v, initialHeartbeatSuccess := c.newVolume(logger, volumeResponse)
	if !initialHeartbeatSuccess {
		return nil, volume.ErrVolumeDoesNotExist
	}
	return v, nil
}

func (c *client) ListVolumes(logger lager.Logger, properties baggageclaim.VolumeProperties) (baggageclaim.Volumes, error) {
	if properties == nil {
		properties = baggageclaim.VolumeProperties{}
	}

	request, err := c.requestGenerator.CreateRequest(baggageclaim.ListVolumes, nil, nil)
	if err != nil {
		return nil, err
	}

	queryString := request.URL.Query()
	for key, val := range properties {
		queryString.Add(key, val)
	}

	request.URL.RawQuery = queryString.Encode()

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return nil, getError(response)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return nil, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumesResponse []baggageclaim.VolumeResponse
	err = json.NewDecoder(response.Body).Decode(&volumesResponse)
	if err != nil {
		return nil, err
	}

	var volumes baggageclaim.Volumes
	for _, vr := range volumesResponse {
		v, initialHeartbeatSuccess := c.newVolume(logger, vr)
		if initialHeartbeatSuccess {
			volumes = append(volumes, v)
		}
	}

	return volumes, nil
}

func (c *client) LookupVolume(logger lager.Logger, handle string) (baggageclaim.Volume, bool, error) {

	volumeResponse, found, err := c.getVolumeResponse(logger, handle)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, found, nil
	}

	v, initialHeartbeatSuccess := c.newVolume(logger, volumeResponse)
	if !initialHeartbeatSuccess {
		return nil, false, nil
	}
	return v, true, nil
}

func (c *client) newVolume(logger lager.Logger, apiVolume baggageclaim.VolumeResponse) (baggageclaim.Volume, bool) {
	volume := &clientVolume{
		logger: logger,

		handle: apiVolume.Handle,
		path:   apiVolume.Path,

		bcClient:     c,
		heartbeating: new(sync.WaitGroup),
		release:      make(chan *time.Duration, 1),
	}

	initialHeartbeatSuccess := volume.startHeartbeating(logger, time.Duration(apiVolume.TTLInSeconds)*time.Second)

	return volume, initialHeartbeatSuccess
}

func (c *client) streamIn(logger lager.Logger, destHandle string, path string, tarContent io.Reader) error {
	request, err := c.requestGenerator.CreateRequest(baggageclaim.StreamIn, rata.Params{
		"handle": destHandle,
	}, tarContent)

	request.URL.RawQuery = url.Values{"path": []string{path}}.Encode()
	if err != nil {
		return err
	}

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return nil
	}
	return getError(response)
}

func (c *client) streamOut(logger lager.Logger, srcHandle string, path string) (io.ReadCloser, error) {
	request, err := c.requestGenerator.CreateRequest(baggageclaim.StreamOut, rata.Params{
		"handle": srcHandle,
	}, nil)

	request.URL.RawQuery = url.Values{"path": []string{path}}.Encode()
	if err != nil {
		return nil, err
	}

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, getError(response)
	}

	return response.Body, nil
}

func getError(response *http.Response) error {
	var errorResponse *api.ErrorResponse
	err := json.NewDecoder(response.Body).Decode(&errorResponse)
	if err != nil {
		return err
	}

	if errorResponse.Message == api.ErrStreamOutNotFound.Error() {
		return baggageclaim.ErrFileNotFound
	}

	if response.StatusCode == 404 {
		return baggageclaim.ErrVolumeNotFound
	}

	return errors.New(errorResponse.Message)
}

func (c *client) getVolumeResponse(logger lager.Logger, handle string) (baggageclaim.VolumeResponse, bool, error) {
	request, err := c.requestGenerator.CreateRequest(baggageclaim.GetVolume, rata.Params{
		"handle": handle,
	}, nil)
	if err != nil {
		return baggageclaim.VolumeResponse{}, false, err
	}

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return baggageclaim.VolumeResponse{}, false, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return baggageclaim.VolumeResponse{}, false, nil
		}

		return baggageclaim.VolumeResponse{}, false, getError(response)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return baggageclaim.VolumeResponse{}, false, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse baggageclaim.VolumeResponse
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return baggageclaim.VolumeResponse{}, false, err
	}

	return volumeResponse, true, nil
}

func (c *client) getVolumeStatsResponse(logger lager.Logger, handle string) (baggageclaim.VolumeStatsResponse, error) {
	request, err := c.requestGenerator.CreateRequest(baggageclaim.GetVolumeStats, rata.Params{
		"handle": handle,
	}, nil)
	if err != nil {
		return baggageclaim.VolumeStatsResponse{}, err
	}

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return baggageclaim.VolumeStatsResponse{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusNotFound {
			return baggageclaim.VolumeStatsResponse{}, nil
		}
		return baggageclaim.VolumeStatsResponse{}, getError(response)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return baggageclaim.VolumeStatsResponse{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeStatsResponse baggageclaim.VolumeStatsResponse
	err = json.NewDecoder(response.Body).Decode(&volumeStatsResponse)
	if err != nil {
		return baggageclaim.VolumeStatsResponse{}, err
	}

	return volumeStatsResponse, nil
}

func (c *client) setTTL(logger lager.Logger, handle string, ttl time.Duration) error {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(baggageclaim.TTLRequest{
		Value: uint(math.Ceil(ttl.Seconds())),
	})

	request, err := c.requestGenerator.CreateRequest(baggageclaim.SetTTL, rata.Params{
		"handle": handle,
	}, buffer)
	if err != nil {
		return err
	}

	request.Header.Add("Content-type", "application/json")

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != 204 {
		return getError(response)
	}

	return nil
}

func (c *client) destroy(logger lager.Logger, handle string) error {
	request, err := c.requestGenerator.CreateRequest(baggageclaim.DestroyVolume, rata.Params{
		"handle": handle,
	}, nil)
	if err != nil {
		return err
	}

	request.Header.Add("Content-type", "application/json")

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != 204 {
		return getError(response)
	}

	return nil
}

func (c *client) setProperty(logger lager.Logger, handle string, propertyName string, propertyValue string) error {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(baggageclaim.PropertyRequest{
		Value: propertyValue,
	})

	request, err := c.requestGenerator.CreateRequest(baggageclaim.SetProperty, rata.Params{
		"handle":   handle,
		"property": propertyName,
	}, buffer)
	if err != nil {
		return err
	}

	response, err := c.httpClient(logger).Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != 204 {
		return getError(response)
	}

	return nil
}
