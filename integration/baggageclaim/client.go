package integration

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

type Client struct {
	requestGenerator *rata.RequestGenerator
}

type VolumeSpec struct {
	Properties volume.Properties
	TTL        uint
}

func NewClient(
	apiURL string,
) *Client {
	return &Client{
		requestGenerator: rata.NewRequestGenerator(apiURL, baggageclaim.Routes),
	}
}

func (c *Client) CreateCOWVolume(parentHandle string, properties volume.Properties) (volume.Volume, error) {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.VolumeRequest{
		Strategy: volume.Strategy{
			"type":   "cow",
			"volume": parentHandle,
		},
		Properties: properties,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return volume.Volume{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 201 {
		return volume.Volume{}, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return volume.Volume{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse volume.Volume
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return volume.Volume{}, err
	}

	return volumeResponse, nil
}

func (c *Client) CreateEmptyVolume(volumeSpec VolumeSpec) (volume.Volume, error) {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.VolumeRequest{
		Strategy: volume.Strategy{
			"type": "empty",
		},
		Properties: volumeSpec.Properties,
		TTL:        &volumeSpec.TTL,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return volume.Volume{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != 201 {
		return volume.Volume{}, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return volume.Volume{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumeResponse volume.Volume
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return volume.Volume{}, err
	}

	return volumeResponse, nil
}

func (c *Client) GetVolumes() (volume.Volumes, error) {
	request, _ := c.requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return volume.Volumes{}, err
	}

	defer response.Body.Close()

	if response.StatusCode != 200 {
		return volume.Volumes{}, fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	if header := response.Header.Get("Content-Type"); header != "application/json" {
		return volume.Volumes{}, fmt.Errorf("unexpected content-type of: %s", header)
	}

	var volumesResponse volume.Volumes
	err = json.NewDecoder(response.Body).Decode(&volumesResponse)
	if err != nil {
		return volume.Volumes{}, err
	}

	return volumesResponse, nil
}

func (c *Client) GetVolume(handle string) (volume.Volume, error) {
	volumesResponse, err := c.GetVolumes()
	if err != nil {
		return volume.Volume{}, err
	}

	for _, volumeResponse := range volumesResponse {
		if volumeResponse.Handle == handle {
			return volumeResponse, nil
		}
	}

	return volume.Volume{}, fmt.Errorf("no volumes matching handle: %s", handle)
}

func (c *Client) SetProperty(handle string, propertyName string, propertyValue string) error {
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

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode != 204 {
		return fmt.Errorf("unexpected response code of: %d", response.StatusCode)
	}

	return response.Body.Close()
}
