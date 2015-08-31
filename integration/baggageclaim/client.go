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

func NewClient(
	apiURL string,
) *Client {
	return &Client{
		requestGenerator: rata.NewRequestGenerator(apiURL, baggageclaim.Routes),
	}
}

func (c *Client) CreateEmptyVolume(properties volume.Properties) (volume.Volume, error) {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.VolumeRequest{
		Strategy: volume.Strategy{
			"type": "empty",
		},
		Properties: properties,
	})

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return volume.Volume{}, err
	}
	defer response.Body.Close()

	var volumeResponse volume.Volume
	err = json.NewDecoder(response.Body).Decode(&volumeResponse)
	if err != nil {
		return volume.Volume{}, err
	}

	return volumeResponse, nil
}

func (c *Client) GetVolume(volumeGUID string) (volume.Volume, error) {

	request, _ := c.requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return volume.Volume{}, err
	}
	defer response.Body.Close()

	var volumesResponse volume.Volumes
	err = json.NewDecoder(response.Body).Decode(&volumesResponse)
	if err != nil {
		return volume.Volume{}, err
	}

	for _, volumeResponse := range volumesResponse {
		if volumeResponse.GUID == volumeGUID {
			return volumeResponse, nil
		}
	}

	return volume.Volume{}, fmt.Errorf("no volumes matching guid: %s", volumeGUID)
}

func (c *Client) SetProperty(volumeGUID string, propertyName string, propertyValue string) error {
	buffer := &bytes.Buffer{}
	json.NewEncoder(buffer).Encode(api.PropertyRequest{
		Value: propertyValue,
	})

	request, err := c.requestGenerator.CreateRequest(baggageclaim.SetProperty, rata.Params{
		"volume":   volumeGUID,
		"property": propertyName,
	}, buffer)
	if err != nil {
		panic(err)
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	return response.Body.Close()
}
