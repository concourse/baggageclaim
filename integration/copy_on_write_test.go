package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tedsuo/rata"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Copy On Write Strategy", func() {
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
		writeData := func(volumePath string) string {
			filename := randSeq(10)
			newFilePath := filepath.Join(volumePath, filename)

			err := ioutil.WriteFile(newFilePath, []byte(filename), 0755)
			Ω(err).NotTo(HaveOccurred())

			return filename
		}

		dataExistsInVolume := func(filename, volumePath string) bool {
			_, err := os.Stat(filepath.Join(volumePath, filename))
			return err == nil
		}

		createVolume := func(request api.VolumeRequest) (volume.Volume, *http.Response) {
			url := fmt.Sprintf("http://localhost:%d", port)
			requestGenerator := rata.NewRequestGenerator(url, baggageclaim.Routes)

			buffer := &bytes.Buffer{}
			err := json.NewEncoder(buffer).Encode(request)
			Ω(err).ShouldNot(HaveOccurred())

			req, err := requestGenerator.CreateRequest(baggageclaim.CreateVolume, nil, buffer)
			Ω(err).ShouldNot(HaveOccurred())

			response, err := http.DefaultClient.Do(req)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response.StatusCode).Should(Equal(http.StatusCreated))

			var volumeResponse volume.Volume
			err = json.NewDecoder(response.Body).Decode(&volumeResponse)
			Ω(err).ShouldNot(HaveOccurred())
			response.Body.Close()

			return volumeResponse, response
		}

		Describe("POST /volumes with strategy: cow", func() {
			It("creates a copy of the volume", func() {
				parentResponse, _ := createVolume(api.VolumeRequest{
					Strategy: volume.Strategy{
						"type": "empty",
					},
				})

				dataInParent := writeData(parentResponse.Path)
				Ω(dataExistsInVolume(dataInParent, parentResponse.Path)).To(BeTrue())

				childResponse, httpResponse := createVolume(api.VolumeRequest{
					Strategy: volume.Strategy{
						"type":   "cow",
						"volume": parentResponse.GUID,
					},
				})

				Ω(httpResponse.StatusCode).Should(Equal(http.StatusCreated))
				Ω(httpResponse.Header.Get("Content-Type")).To(Equal("application/json"))

				Ω(dataExistsInVolume(dataInParent, childResponse.Path)).To(BeTrue())

				newDataInParent := writeData(parentResponse.Path)
				Ω(dataExistsInVolume(newDataInParent, parentResponse.Path)).To(BeTrue())
				Ω(dataExistsInVolume(newDataInParent, childResponse.Path)).To(BeFalse())

				dataInChild := writeData(childResponse.Path)
				Ω(dataExistsInVolume(dataInChild, childResponse.Path)).To(BeTrue())
				Ω(dataExistsInVolume(dataInChild, parentResponse.Path)).To(BeFalse())
			})
		})
	})
})

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
