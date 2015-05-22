package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/mattermaster/api"
	"github.com/concourse/mattermaster/volume"
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
		repo := volume.NewRepository(logger, volumeDir)
		server = api.NewVolumeServer(logger, repo)
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

		Context("when there are no volumes", func() {
			It("returns an empty array", func() {
				Ω(recorder.Body).Should(MatchJSON(`[]`))
			})
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
		var (
			recorder *httptest.ResponseRecorder
			body     io.ReadWriter
		)

		BeforeEach(func() {
			body = &bytes.Buffer{}
			json.NewEncoder(body).Encode(api.VolumeRequest{
				Strategy: volume.Strategy{
					"type": "empty",
				},
			})
		})

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)

			server.CreateVolume(recorder, request)
		})

		Context("when a new directory can be created", func() {
			It("writes a nice JSON response", func() {
				Ω(recorder.Body).Should(ContainSubstring(`"path":`))
				Ω(recorder.Body).Should(ContainSubstring(`"guid":`))
			})
		})

		Context("when invalid JSON is submitted", func() {
			BeforeEach(func() {
				body = bytes.NewBufferString("{{{{{{")
			})

			It("returns a 400 Bad Request response", func() {
				Ω(recorder.Code).Should(Equal(http.StatusBadRequest))
			})

			It("writes a nice JSON response", func() {
				Ω(recorder.Body).Should(ContainSubstring(`"error":`))
			})

			It("does not create a volume", func() {
				getRecorder := httptest.NewRecorder()
				getReq, _ := http.NewRequest("GET", "/volumes", nil)
				server.GetVolumes(getRecorder, getReq)
				Ω(getRecorder.Body).Should(MatchJSON("[]"))
			})
		})

		Context("when no strategy is submitted", func() {
			BeforeEach(func() {
				body = bytes.NewBufferString("{}")
			})

			It("returns a 422 Unprocessable Entity response", func() {
				Ω(recorder.Code).Should(Equal(422))
			})

			It("writes a nice JSON response", func() {
				Ω(recorder.Body).Should(ContainSubstring(`"error":`))
			})

			It("does not create a volume", func() {
				getRecorder := httptest.NewRecorder()
				getReq, _ := http.NewRequest("GET", "/volumes", nil)
				server.GetVolumes(getRecorder, getReq)
				Ω(getRecorder.Body).Should(MatchJSON("[]"))
			})
		})

		Context("when an unrecognized strategy is submitted", func() {
			BeforeEach(func() {
				body = &bytes.Buffer{}
				json.NewEncoder(body).Encode(api.VolumeRequest{
					Strategy: volume.Strategy{
						"type": "grime",
					},
				})
			})

			It("returns a 422 Unprocessable Entity response", func() {
				Ω(recorder.Code).Should(Equal(422))
			})

			It("writes a nice JSON response", func() {
				Ω(recorder.Body).Should(ContainSubstring(`"error":`))
			})

			It("does not create a volume", func() {
				getRecorder := httptest.NewRecorder()
				getReq, _ := http.NewRequest("GET", "/volumes", nil)
				server.GetVolumes(getRecorder, getReq)
				Ω(getRecorder.Body).Should(MatchJSON("[]"))
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
