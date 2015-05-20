package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/rata"

	"github.com/concourse/mattermaster"
	"github.com/concourse/mattermaster/api"
)

var _ = Describe("Matter Master", func() {
	var (
		runner    *matterMasterRunner
		port      int
		volumeDir string
	)

	BeforeEach(func() {
		var err error

		port = 7788 + GinkgoParallelNode()
		volumeDir, err = ioutil.TempDir("", fmt.Sprintf("mattermaster_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		runner = newRunner(matterMasterPath, port, volumeDir)
		runner.start()
	})

	AfterEach(func() {
		runner.stop()
		runner.cleanup()
	})

	Describe("API", func() {
		createVolume := func() (api.VolumeResponse, *http.Response) {
			var err error
			url := fmt.Sprintf("http://localhost:%d", port)
			requestGenerator := rata.NewRequestGenerator(url, mattermaster.Routes)
			request, err := requestGenerator.CreateRequest(mattermaster.CreateVolume, nil, nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err := http.DefaultClient.Do(request)
			Ω(err).ShouldNot(HaveOccurred())

			var volumeResponse api.VolumeResponse

			err = json.NewDecoder(response.Body).Decode(&volumeResponse)
			Ω(err).ShouldNot(HaveOccurred())
			response.Body.Close()

			return volumeResponse, response
		}

		Describe("POST /volumes", func() {
			var (
				response       *http.Response
				volumeResponse api.VolumeResponse
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
				getVolumeResponse api.VolumesResponse
			)

			getVolumes := func() (api.VolumesResponse, *http.Response) {
				var err error
				url := fmt.Sprintf("http://localhost:%d", port)
				requestGenerator := rata.NewRequestGenerator(url, mattermaster.Routes)
				request, err := requestGenerator.CreateRequest(mattermaster.GetVolumes, nil, nil)
				Ω(err).ShouldNot(HaveOccurred())

				response, err := http.DefaultClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())

				var getVolumeResponse api.VolumesResponse

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
				var createVolumeResponse api.VolumeResponse

				BeforeEach(func() {
					createVolumeResponse, _ = createVolume()
				})

				It("returns it", func() {
					Ω(getVolumeResponse).Should(ConsistOf(createVolumeResponse))
				})
			})

		})
	})
})
