package integration_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/concourse/mattermaster"
	"github.com/concourse/mattermaster/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"github.com/tedsuo/rata"
)

var _ = Describe("Matter Master", func() {
	var (
		process   ifrit.Process
		port      int
		volumeDir string
	)

	BeforeEach(func() {
		var err error

		port = 7788 + GinkgoParallelNode()
		volumeDir, err = ioutil.TempDir("", fmt.Sprintf("mattermaster_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		runner := ginkgomon.New(ginkgomon.Config{
			Name: "mattermaster",
			Command: exec.Command(
				matterMasterPath,
				"-listenPort", strconv.Itoa(port),
				"-volumeDir", volumeDir,
			),
			StartCheck: "mattermaster.listening",
		})

		process = ginkgomon.Invoke(runner)
	})

	AfterEach(func() {
		process.Signal(os.Kill)
		Eventually(process.Wait()).Should(Receive())

		err := os.RemoveAll(volumeDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("API", func() {
		Describe("POST /volumes", func() {
			var (
				response             *http.Response
				createVolumeResponse api.CreateVolumeResponse
			)

			createVolume := func() (api.CreateVolumeResponse, *http.Response) {
				var err error
				url := fmt.Sprintf("http://localhost:%d", port)
				requestGenerator := rata.NewRequestGenerator(url, mattermaster.Routes)
				request, err := requestGenerator.CreateRequest(mattermaster.CreateVolume, nil, nil)
				Ω(err).ShouldNot(HaveOccurred())

				response, err := http.DefaultClient.Do(request)
				Ω(err).ShouldNot(HaveOccurred())

				var createVolumeResponse api.CreateVolumeResponse

				err = json.NewDecoder(response.Body).Decode(&createVolumeResponse)
				Ω(err).ShouldNot(HaveOccurred())
				response.Body.Close()

				return createVolumeResponse, response
			}

			JustBeforeEach(func() {
				createVolumeResponse, response = createVolume()
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
					createdDir = createVolumeResponse.Path
				})

				It("is in the volume dir", func() {
					Ω(createdDir).Should(HavePrefix(volumeDir))
				})

				It("creates the directory", func() {
					Ω(createdDir).Should(BeADirectory())
				})

				Context("on a second request", func() {
					var (
						secondCreatedDir string
					)

					JustBeforeEach(func() {
						secondCreateVolumeResponse, _ := createVolume()

						secondCreatedDir = secondCreateVolumeResponse.Path
					})

					It("creates a new directory", func() {
						Ω(createdDir).ShouldNot(Equal(secondCreatedDir))
					})
				})
			})
		})
	})
})
