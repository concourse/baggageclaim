package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/rata"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Restarting", func() {
	var (
		runner *BaggageClaimRunner
	)

	BeforeEach(func() {

		runner = NewRunner(baggageClaimPath)
		runner.Start()
	})

	AfterEach(func() {
		runner.Stop()
		runner.Cleanup()
	})

	createVolume := func() (volume.Volume, *http.Response) {
		var err error
		url := fmt.Sprintf("http://localhost:%d", runner.Port())
		requestGenerator := rata.NewRequestGenerator(url, baggageclaim.Routes)

		buffer := &bytes.Buffer{}
		json.NewEncoder(buffer).Encode(api.VolumeRequest{
			Strategy: volume.Strategy{
				"type": "empty",
			},
		})

		request, err := requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
		Ω(err).ShouldNot(HaveOccurred())

		response, err := http.DefaultClient.Do(request)
		Ω(err).ShouldNot(HaveOccurred())

		var volumeResponse volume.Volume

		err = json.NewDecoder(response.Body).Decode(&volumeResponse)
		Ω(err).ShouldNot(HaveOccurred())
		response.Body.Close()

		return volumeResponse, response
	}

	getVolumes := func() (volume.Volumes, *http.Response) {
		var err error
		url := fmt.Sprintf("http://localhost:%d", runner.Port())
		requestGenerator := rata.NewRequestGenerator(url, baggageclaim.Routes)
		request, err := requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)
		Ω(err).ShouldNot(HaveOccurred())

		response, err := http.DefaultClient.Do(request)
		Ω(err).ShouldNot(HaveOccurred())

		var getVolumeResponse volume.Volumes

		err = json.NewDecoder(response.Body).Decode(&getVolumeResponse)
		Ω(err).ShouldNot(HaveOccurred())
		response.Body.Close()

		return getVolumeResponse, response
	}

	It("can get volumes after the process restarts", func() {
		volumeResponse, _ := createVolume()
		volumes, _ := getVolumes()
		Ω(volumes).Should(ConsistOf(volumeResponse))

		runner.Bounce()

		volumesAfterRestart, _ := getVolumes()
		Ω(volumesAfterRestart).Should(Equal(volumes))
	})
})
