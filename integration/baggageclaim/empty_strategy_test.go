package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/rata"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Empty Strategy", func() {
	var (
		runner    *BaggageClaimRunner
		port      int
		volumeDir string
	)

	BeforeEach(func() {
		var err error

		port = 7788 + GinkgoParallelNode()
		volumeDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		runner = NewRunner(baggageClaimPath, port, volumeDir)
		runner.start()
	})

	AfterEach(func() {
		runner.stop()
		runner.cleanup()
	})

	Describe("API", func() {
		properties := volume.Properties{
			"name": "value",
		}

		createVolume := func() (volume.Volume, *http.Response) {
			url := fmt.Sprintf("http://localhost:%d", port)
			requestGenerator := rata.NewRequestGenerator(url, baggageclaim.Routes)

			buffer := &bytes.Buffer{}
			json.NewEncoder(buffer).Encode(api.VolumeRequest{
				Strategy: volume.Strategy{
					"type": "empty",
				},
				Properties: properties,
			})

			request, err := requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
			Ω(err).ShouldNot(HaveOccurred())

			response, err := http.DefaultClient.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(response.StatusCode).Should(Equal(http.StatusCreated))

			var volumeResponse volume.Volume
			err = json.NewDecoder(response.Body).Decode(&volumeResponse)
			Ω(err).ShouldNot(HaveOccurred())
			response.Body.Close()

			return volumeResponse, response
		}

		Describe("POST /volumes", func() {
			var (
				response       *http.Response
				volumeResponse volume.Volume
			)

			JustBeforeEach(func() {
				volumeResponse, response = createVolume()
			})

			It("has a response code of 201 CREATED", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusCreated))
			})

			It("has a JSON Content-type header", func() {
				Ω(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			Describe("created directory", func() {
				var (
					createdDir string
				)

				JustBeforeEach(func() {
					createdDir = volumeResponse.Path
				})

				It("is in the volume dir", func() {
					Ω(createdDir).Should(HavePrefix(volumeDir))
				})

				It("creates the directory", func() {
					Ω(createdDir).Should(BeADirectory())
				})

				Context("on a second request", func() {
					var (
						secondCreatedDir  string
						secondCreatedGUID string
					)

					JustBeforeEach(func() {
						secondCreateVolumeResponse, _ := createVolume()

						secondCreatedDir = secondCreateVolumeResponse.Path
						secondCreatedGUID = secondCreateVolumeResponse.GUID
					})

					It("creates a new directory", func() {
						Ω(createdDir).ShouldNot(Equal(secondCreatedDir))
					})

					It("creates a new GUID", func() {
						Ω(volumeResponse.GUID).ShouldNot(Equal(secondCreatedGUID))
					})
				})
			})
		})

		Describe("GET /volumes", func() {
			var (
				response          *http.Response
				getVolumeResponse volume.Volumes
			)

			getVolumes := func() (volume.Volumes, *http.Response) {
				var err error
				url := fmt.Sprintf("http://localhost:%d", port)
				requestGenerator := rata.NewRequestGenerator(url, baggageclaim.Routes)
				request, err := requestGenerator.CreateRequest(baggageclaim.GetVolumes, nil, nil)
				Ω(err).ShouldNot(HaveOccurred())

				response, err := http.DefaultClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response.StatusCode).Should(Equal(http.StatusOK))

				var getVolumeResponse volume.Volumes

				err = json.NewDecoder(response.Body).Decode(&getVolumeResponse)
				Ω(err).ShouldNot(HaveOccurred())
				response.Body.Close()

				return getVolumeResponse, response
			}

			JustBeforeEach(func() {
				getVolumeResponse, response = getVolumes()
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("has a JSON Content-type header", func() {
				Ω(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns an empty response", func() {
				Ω(getVolumeResponse).Should(BeEmpty())
			})

			Context("when a volume has been created", func() {
				var createVolumeResponse volume.Volume

				BeforeEach(func() {
					createVolumeResponse, _ = createVolume()
				})

				It("returns it", func() {
					Ω(getVolumeResponse[0].Properties).Should(Equal(
						properties,
					))

					Ω(getVolumeResponse).Should(ConsistOf(createVolumeResponse))
				})
			})
		})
	})
})
