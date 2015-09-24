package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
	"github.com/tedsuo/rata"
)

type Client interface {
	baggageclaim.Client
}

type client struct {
	requestGenerator *rata.RequestGenerator
	httpClient       *http.Client
}

func New(
	apiURL string,
) Client {
	return &client{
		requestGenerator: rata.NewRequestGenerator(apiURL, baggageclaim.Routes),
		httpClient: &http.Client{
			Transport: &http.Transport{},
		},
	}
}

func (c *client) CreateCOWVolume(volumeSpec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.VolumeRequest{
		Strategy: volume.Strategy{
			"type":   "cow",
			"volume": volumeSpec.ParentHandle,
		},
		TTLInSeconds: volumeSpec.TTLInSeconds,
		Properties:   volumeSpec.Properties,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return nil, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse volume.Volume
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return nil, err
	}

	return NewVolume(c, volumeResponse), nil
}

func (c *client) CreateEmptyVolume(volumeSpec baggageclaim.VolumeSpec) (baggageclaim.Volume, error) {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.VolumeRequest{
		Strategy: volume.Strategy{
			"type": "empty",
		},
		Properties:   volumeSpec.Properties,
		TTLInSeconds: volumeSpec.TTLInSeconds,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != 201 {
		return nil, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return nil, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse volume.Volume
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return nil, err
	}

	return NewVolume(c, volumeResponse), nil
}

func (c *client) FindVolumes(properties baggageclaim.VolumeProperties) (baggageclaim.Volumes, error) {
	request, _ := c.requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)

	queryString := request.URL.Query()
	for key, val := range properties {
		queryString.Add(key, val)
	}
	request.URL.RawQuery = queryString.Encode()

	response, err := c.httpClient.Do(request)
	if err != nil {
		return baggageclaim.Volumes{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return baggageclaim.Volumes{}, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return baggageclaim.Volumes{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumesResponse volume.Volumes
	err = json.NewDecoder(response.Body).Decode(&volumesResponse)
	if err != nil {
		return baggageclaim.Volumes{}, err
	}

	return NewVolumes(c, volumesResponse), nil
}

func (c *client) GetVolumes() (baggageclaim.Volumes, error) {
	request, _ := c.requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return baggageclaim.Volumes{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return baggageclaim.Volumes{}, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return baggageclaim.Volumes{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumesResponse volume.Volumes
	err = json.NewDecoder(response.Body).Decode(&volumesResponse)
	if err != nil {
		return baggageclaim.Volumes{}, err
	}

	return NewVolumes(c, volumesResponse), nil
}

func (c *client) GetVolume(handle string) (baggageclaim.Volume, error) {
	volumesResponse, err := c.GetVolumes()
	if err != nil {
		return nil, err
	}

	for _, volumeResponse := range volumesResponse {
		if volumeResponse.Handle() == handle {
			return volumeResponse, nil
		}
	}

	return nil, fmt.Errorf("no volumes matching handle: %s", handle)
}

func (c *client) SetTTL(handle string, ttl uint) error {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.TTLRequest{
		Value: ttl,
	})

	request, err := c.requestGenerator.CreateRequest(baggageclaim.SetTTL, rata.Params{
		"handle": handle,
	}, buffer)
	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode == 404 {
		return baggageclaim.ErrVolumeNotFound
	}

	if response.StatusCode != 204 {
		return fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	return nil
}

func (c *client) SetProperty(handle string, propertyName string, propertyValue string) error {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.PropertyRequest{
		Value: propertyValue,
	})

	request, err := c.requestGenerator.CreateRequest(baggageclaim.SetProperty, rata.Params{
		"handle":   handle,
		"property": propertyName,
	}, buffer)
	if err != nil {
		panic(err)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode == 404 {
		return baggageclaim.ErrVolumeNotFound
	}

	if response.StatusCode != 204 {
		return fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	return nil
}
