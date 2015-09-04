package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Volume Server Locking", func() {
	var (
		handler http.Handler

		volumeDir  string
		tempDir    string
		fakeDriver *fakes.FakeDriver
		fakeLocker *fakes.FakeLocker
	)

	BeforeEach(func() {
		var err error
		fakeDriver = new(fakes.FakeDriver)
		fakeLocker = new(fakes.FakeLocker)
		tempDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		volumeDir = tempDir
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-server")
		repo := volume.NewRepository(logger, fakeDriver, fakeLocker, volumeDir, volume.TTL(60))

		var err error
		handler, err = api.NewHandler(logger, repo)
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	FIt("cannot set properties on a volume that's having it's TTL set", func() {
		body := &bytes.Buffer{}
		json.NewEncoder(body).Encode(api.VolumeRequest{
			Strategy: volume.Strategy{
				"type": "empty",
			},
			Properties: volume.Properties{
				"property-name": "property-value",
			},
		})

		recorder := httptest.NewRecorder()
		request, _ := http.NewRequest("POST", "/volumes", body)
		handler.ServeHTTP(recorder, request)
		Ω(recorder.Code).Should(Equal(201))

		var createdVolume volume.Volume
		err := json.NewDecoder(recorder.Body).Decode(&createdVolume)
		Ω(err).ShouldNot(HaveOccurred())
		fmt.Println("CREATED VOLUME: ", createdVolume)

		err = json.NewEncoder(body).Encode(api.PropertyRequest{
			Value: "other-val",
		})
		Ω(err).ShouldNot(HaveOccurred())
		recorder = httptest.NewRecorder()
		request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/properties/property-name", createdVolume.Handle), body)
		handler.ServeHTTP(recorder, request)
		Ω(recorder.Code).Should(Equal(http.StatusNoContent))
		Ω(recorder.Body.String()).Should(BeEmpty())

		err = json.NewEncoder(body).Encode(api.TTLRequest{
			Value: 1,
		})
		recorder = httptest.NewRecorder()
		request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/ttl", createdVolume.Handle), body)
		handler.ServeHTTP(recorder, request)
		Ω(recorder.Code).Should(Equal(http.StatusNoContent))
		Ω(recorder.Body.String()).Should(BeEmpty())

	})

	XIt("cannot set properties on a volume that's having it's properties set", func() {
	})

	XIt("cannot set a TTL on a volume that's having it's TTL set", func() {
	})

	XIt("cannot set properties on a volume that's being deleted", func() {
	})

	XIt("cannot set a TTL on a volume that's being deleted", func() {
	})

	XIt("can set properties on 2 different volumes at the same time", func() {
	})

	XIt("can set TTL's on 2 different volumes at the same time", func() {
	})

	XIt("can delete 2 volumes at the same time", func() {
	})

	XIt("cannot create a child volume if the parent is being deleted", func() {
	})

})
