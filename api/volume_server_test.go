package api_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/mattermaster/api"
)

var _ = Describe("Volume Server", func() {
	var (
		server *api.VolumeServer

		volumeDir string
		tempDir   string
	)

	BeforeEach(func() {
		var err error

		tempDir, err = ioutil.TempDir("", fmt.Sprintf("mattermaster_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		volumeDir = tempDir
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-server")
		server = api.NewVolumeServer(logger, volumeDir)
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	Describe("listing the volumes", func() {
		var recorder *httptest.ResponseRecorder

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes", nil)

			server.GetVolumes(recorder, request)
		})

		Context("when the volumes directory is all messed up", func() {
			BeforeEach(func() {
				volumeDir = "/this/cannot/be/read/from"
			})

			It("writes a 500 InternalServerError", func() {
				Ω(recorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("writes a useful JSON error", func() {
				Ω(recorder.Body).Should(MatchJSON(`{"error":"failed to list volumes"}`))
			})
		})
	})

	Describe("creating a volume", func() {
		var recorder *httptest.ResponseRecorder

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", nil)

			server.CreateVolume(recorder, request)
		})

		Context("when a new directory can be created", func() {
			It("writes a nice JSON response", func() {
				Ω(recorder.Body).Should(ContainSubstring(`"path":`))
				Ω(recorder.Body).Should(ContainSubstring(`"guid":`))
			})
		})

		Context("when a new directory cannot be created", func() {
			BeforeEach(func() {
				volumeDir = "/dev/null"
			})

			It("writes a 500 InternalServerError", func() {
				Ω(recorder.Code).Should(Equal(http.StatusInternalServerError))
			})

			It("writes a useful JSON error", func() {
				Ω(recorder.Body).Should(MatchJSON(`{"error":"failed to create volume"}`))
			})
		})
	})
})
