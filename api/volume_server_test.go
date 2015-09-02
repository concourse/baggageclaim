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
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
)

var _ = Describe("Volume Server", func() {
	var (
		handler http.Handler

		volumeDir string
		tempDir   string
	)

	BeforeEach(func() {
		var err error

		tempDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		volumeDir = tempDir
	})

	JustBeforeEach(func() {

		logger := lagertest.NewTestLogger("volume-server")
		repo := volume.NewRepository(logger, volumeDir, &driver.NaiveDriver{}, volume.TTL(60))

		var err error
		handler, err = api.NewHandler(logger, repo)
		Ω(err).ShouldNot(HaveOccurred())
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

			handler.ServeHTTP(recorder, request)
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

	Describe("querying for volumes with properties", func() {
		props := volume.Properties{
			"property-query": "value",
		}

		It("finds volumes that have a property", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(api.VolumeRequest{
				Strategy: volume.Strategy{
					"type": "empty",
				},
				Properties: props,
			})
			Ω(err).ShouldNot(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(201))

			body.Reset()
			err = json.NewEncoder(body).Encode(api.VolumeRequest{
				Strategy: volume.Strategy{
					"type": "empty",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-query=value", nil)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(volumes).Should(HaveLen(1))
		})

		It("returns an error if an invalid set of properties are specified", func() {
			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes?property-query=value&property-query=another-value", nil)
			handler.ServeHTTP(recorder, request)

			Ω(recorder.Code).Should(Equal(422))
		})
	})

	Describe("updating a volume", func() {
		It("can have it's properties updated", func() {
			recorder := httptest.NewRecorder()
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(api.VolumeRequest{
				Strategy: volume.Strategy{
					"type": "empty",
				},
				Properties: volume.Properties{
					"property-name": "property-val",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=property-val", nil)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(volumes).Should(HaveLen(1))

			err = json.NewEncoder(body).Encode(api.PropertyRequest{
				Value: "other-val",
			})

			Ω(err).ShouldNot(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/properties/property-name", volumes[0].GUID), body)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(http.StatusNoContent))
			Ω(recorder.Body.String()).Should(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=other-val", nil)
			handler.ServeHTTP(recorder, request)
			Ω(recorder.Code).Should(Equal(200))

			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(volumes).Should(HaveLen(1))
		})
	})

	Describe("creating a volume", func() {
		var (
			recorder *httptest.ResponseRecorder
			body     io.ReadWriter
		)

		JustBeforeEach(func() {
			recorder = httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)

			handler.ServeHTTP(recorder, request)
		})

		Context("when no TTL is given", func() {
			BeforeEach(func() {
				body = &bytes.Buffer{}
				json.NewEncoder(body).Encode(api.VolumeRequest{
					Strategy: volume.Strategy{
						"type": "empty",
					},
				})
			})

			It("sets it to the default", func() {
				var response volume.Volume
				err := json.NewDecoder(recorder.Body).Decode(&response)
				Ω(err).ShouldNot(HaveOccurred())

				recorder = httptest.NewRecorder()
				request, _ := http.NewRequest("GET", "/volumes", nil)
				handler.ServeHTTP(recorder, request)
				Ω(recorder.Code).Should(Equal(200))

				var volumes volume.Volumes
				err = json.NewDecoder(recorder.Body).Decode(&volumes)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(volumes).Should(HaveLen(1))
				Ω(volumes[0].TTL).Should(Equal(volume.TTL(60)))
			})
		})

		Context("when there are properties given", func() {
			var properties volume.Properties

			Context("with valid properties", func() {
				BeforeEach(func() {
					properties = volume.Properties{
						"property-name": "property-value",
					}

					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(api.VolumeRequest{
						Strategy: volume.Strategy{
							"type": "empty",
						},
						Properties: properties,
					})
				})

				It("creates the properties file", func() {
					var response volume.Volume
					err := json.NewDecoder(recorder.Body).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					propertiesPath := filepath.Join(volumeDir, response.GUID, "properties.json")
					Ω(propertiesPath).Should(BeAnExistingFile())

					propertiesContents, err := ioutil.ReadFile(propertiesPath)
					Ω(err).ShouldNot(HaveOccurred())

					var storedProperties volume.Properties
					err = json.Unmarshal(propertiesContents, &storedProperties)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(storedProperties).Should(Equal(properties))
				})

				It("returns the properties in the response", func() {
					Ω(recorder.Body).Should(ContainSubstring(`"property-name":"property-value"`))
				})
			})
		})

		Context("when there are no properties given", func() {
			BeforeEach(func() {
				body = &bytes.Buffer{}
				json.NewEncoder(body).Encode(api.VolumeRequest{
					Strategy: volume.Strategy{
						"type": "empty",
					},
				})
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
					handler.ServeHTTP(getRecorder, getReq)
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
					handler.ServeHTTP(getRecorder, getReq)
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
					handler.ServeHTTP(getRecorder, getReq)
					Ω(getRecorder.Body).Should(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow but not parent volume is provided", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(api.VolumeRequest{
						Strategy: volume.Strategy{
							"type": "cow",
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
					handler.ServeHTTP(getRecorder, getReq)
					Ω(getRecorder.Body).Should(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow and the parent volume does not exist", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(api.VolumeRequest{
						Strategy: volume.Strategy{
							"type":   "cow",
							"volume": "#pain",
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
					handler.ServeHTTP(getRecorder, getReq)
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
})
