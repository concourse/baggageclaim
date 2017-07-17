package api_test

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/uidgid"
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
		Expect(err).NotTo(HaveOccurred())

		// ioutil.TempDir creates it 0700; we need public readability for
		// unprivileged StreamIn
		err = os.Chmod(tempDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		volumeDir = tempDir
	})

	JustBeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-server")

		fs, err := volume.NewFilesystem(&driver.NaiveDriver{}, volumeDir)
		Expect(err).NotTo(HaveOccurred())

		var privilegedNamespacer, unprivilegedNamespacer uidgid.Namespacer
		if runtime.GOOS == "linux" {
			privilegedNamespacer = &uidgid.UidNamespacer{
				Translator: uidgid.NewTranslator(uidgid.NewPrivilegedMapper()),
				Logger:     logger.Session("uid-namespacer"),
			}

			unprivilegedNamespacer = &uidgid.UidNamespacer{
				Translator: uidgid.NewTranslator(uidgid.NewUnprivilegedMapper()),
				Logger:     logger.Session("uid-namespacer"),
			}
		} else {
			privilegedNamespacer = &uidgid.NoopNamespacer{}
			unprivilegedNamespacer = &uidgid.NoopNamespacer{}
		}

		repo := volume.NewRepository(
			logger,
			fs,
			volume.NewLockManager(),
			privilegedNamespacer,
			unprivilegedNamespacer,
		)

		strategerizer := volume.NewStrategerizer()

		handler, err = api.NewHandler(logger, strategerizer, repo)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
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
				Expect(recorder.Body).To(MatchJSON(`[]`))
			})
		})
	})

	Describe("querying for volumes with properties", func() {
		props := baggageclaim.VolumeProperties{
			"property-query": "value",
		}

		It("finds volumes that have a property", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Properties: props,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			body.Reset()
			err = json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "another-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-query=value", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumes).To(HaveLen(1))
		})

		It("returns an error if an invalid set of properties are specified", func() {
			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("GET", "/volumes?property-query=value&property-query=another-value", nil)
			handler.ServeHTTP(recorder, request)

			Expect(recorder.Code).To(Equal(422))
		})
	})

	Describe("streaming tar files into volumes", func() {
		var (
			myVolume     volume.Volume
			tarBuffer    *(bytes.Buffer)
			isPrivileged bool
		)

		JustBeforeEach(func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Privileged: isPrivileged,
			})
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", "/volumes", body)
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			err = json.NewDecoder(recorder.Body).Decode(&myVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when tar file is valid", func() {
			BeforeEach(func() {
				tarBuffer = new(bytes.Buffer)
				tarWriter := tar.NewWriter(tarBuffer)

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "some-file",
					Mode: 0600,
					Size: int64(len("file-content")),
				})
				Expect(err).NotTo(HaveOccurred())
				_, err = tarWriter.Write([]byte("file-content"))
				Expect(err).NotTo(HaveOccurred())

				err = tarWriter.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			It("extracts the tar stream into the volume's DataPath", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(204))

				tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
				Expect(tarContentsPath).To(BeAnExistingFile())

				Expect(ioutil.ReadFile(tarContentsPath)).To(Equal([]byte("file-content")))
			})

			Context("when volume is not privileged", func() {
				BeforeEach(func() {
					isPrivileged = false
				})

				It("namespaces volume path", func() {
					if runtime.GOOS != "linux" {
						Skip("only runs somewhere we can run privileged")
						return
					}

					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarInfoPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarInfoPath).To(BeAnExistingFile())

					stat, err := os.Stat(tarInfoPath)
					Expect(err).ToNot(HaveOccurred())

					maxUID := uidgid.MustGetMaxValidUID()
					maxGID := uidgid.MustGetMaxValidGID()

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
					Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
				})
			})

			Context("when volume privileged", func() {
				BeforeEach(func() {
					isPrivileged = true
				})

				It("namespaces volume path", func() {
					if runtime.GOOS != "linux" {
						Skip("only runs somewhere we can run privileged")
						return
					}

					request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, request)
					Expect(recorder.Code).To(Equal(204))

					tarInfoPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
					Expect(tarInfoPath).To(BeAnExistingFile())

					stat, err := os.Stat(tarInfoPath)
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(0)))
					Expect(sysStat.Gid).To(Equal(uint32(0)))
				})
			})
		})

		Context("when the tar stream is invalid", func() {
			BeforeEach(func() {
				tarBuffer = new(bytes.Buffer)
				tarBuffer.Write([]byte("This is an invalid tar file!"))
			})

			It("returns 400 when err is exitError", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", myVolume.Handle), tarBuffer)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(400))
			})
		})

		It("returns 404 when volume is not found", func() {
			tarBuffer = new(bytes.Buffer)
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in", "invalid-handle"), tarBuffer)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))
		})
	})

	Describe("streaming tar out of a volume", func() {
		var (
			myVolume  volume.Volume
			tarBuffer *(bytes.Buffer)
		)

		BeforeEach(func() {
			tarBuffer = new(bytes.Buffer)
		})

		JustBeforeEach(func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
			})
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("POST", "/volumes", body)
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			err = json.NewDecoder(recorder.Body).Decode(&myVolume)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 404 when source path is invalid", func() {
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "bogus-path"), nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))

			var responseError *api.ErrorResponse
			err := json.NewDecoder(recorder.Body).Decode(&responseError)
			Expect(err).NotTo(HaveOccurred())
			Expect(responseError.Message).To(Equal("no such file or directory"))
		})

		Context("when streaming a file", func() {
			BeforeEach(func() {
				tarWriter := tar.NewWriter(tarBuffer)

				err := tarWriter.WriteHeader(&tar.Header{
					Name: "some-file",
					Mode: 0600,
					Size: int64(len("file-content")),
				})
				Expect(err).NotTo(HaveOccurred())
				_, err = tarWriter.Write([]byte("file-content"))
				Expect(err).NotTo(HaveOccurred())

				err = tarWriter.Close()
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				streamInRequest, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
				streamInRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamInRecorder, streamInRequest)
				Expect(streamInRecorder.Code).To(Equal(204))

				tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path", "some-file")
				Expect(tarContentsPath).To(BeAnExistingFile())

				Expect(ioutil.ReadFile(tarContentsPath)).To(Equal([]byte("file-content")))
			})

			It("creates a tar", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))

				unpackedDir := filepath.Join(tempDir, "unpacked-dir")
				err := os.MkdirAll(unpackedDir, os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				defer os.RemoveAll(unpackedDir)

				cmd := exec.Command("tar", "-x", "-C", unpackedDir)
				cmd.Stdin = recorder.Body
				err = cmd.Run()
				Expect(err).NotTo(HaveOccurred())

				fileInfo, err := os.Stat(filepath.Join(unpackedDir, "some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeFalse())
				Expect(fileInfo.Size()).To(Equal(int64(len("file-content"))))

				contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "./some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("file-content"))
			})
		})

		Context("when streaming a directory", func() {
			var tarDir string

			BeforeEach(func() {
				tarDir = filepath.Join(tempDir, "tar-dir")

				err := os.MkdirAll(filepath.Join(tarDir, "sub"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tarDir, "sub", "some-file"), []byte("some-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tarDir, "other-file"), []byte("other-file-content"), os.ModePerm)
				Expect(err).NotTo(HaveOccurred())

				cmd := exec.Command("tar", "-c", ".")
				cmd.Dir = tarDir
				tarBytes, err := cmd.Output()
				Expect(err).NotTo(HaveOccurred())

				tarBuffer = bytes.NewBuffer(tarBytes)
			})

			JustBeforeEach(func() {
				streamInRequest, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-in?path=%s", myVolume.Handle, "dest-path"), tarBuffer)
				streamInRecorder := httptest.NewRecorder()
				handler.ServeHTTP(streamInRecorder, streamInRequest)
				Expect(streamInRecorder.Code).To(Equal(204))

				tarContentsPath := filepath.Join(volumeDir, "live", myVolume.Handle, "volume", "dest-path")
				Expect(tarContentsPath).To(BeADirectory())
			})

			It("creates a tar", func() {
				request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out?path=%s", myVolume.Handle, "dest-path"), nil)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				Expect(recorder.Code).To(Equal(200))

				unpackedDir := filepath.Join(tempDir, "unpacked-dir")
				err := os.MkdirAll(unpackedDir, os.ModePerm)
				Expect(err).NotTo(HaveOccurred())
				defer os.RemoveAll(unpackedDir)

				cmd := exec.Command("tar", "-x", "-C", unpackedDir)
				cmd.Stdin = recorder.Body
				err = cmd.Run()
				Expect(err).NotTo(HaveOccurred())

				fileInfo, err := os.Stat(filepath.Join(unpackedDir, "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeFalse())
				Expect(fileInfo.Size()).To(Equal(int64(len("other-file-content"))))

				contents, err := ioutil.ReadFile(filepath.Join(unpackedDir, "other-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("other-file-content"))

				dirInfo, err := os.Stat(filepath.Join(unpackedDir, "sub"))
				Expect(err).NotTo(HaveOccurred())
				Expect(dirInfo.IsDir()).To(BeTrue())

				fileInfo, err = os.Stat(filepath.Join(unpackedDir, "sub/some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(fileInfo.IsDir()).To(BeFalse())
				Expect(fileInfo.Size()).To(Equal(int64(len("some-file-content"))))

				contents, err = ioutil.ReadFile(filepath.Join(unpackedDir, "sub/some-file"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal("some-file-content"))
			})
		})

		It("returns 404 when volume is not found", func() {
			request, _ := http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/stream-out", "invalid-handle"), nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(404))
		})
	})

	Describe("updating a volume", func() {
		It("can have it's properties updated", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				Properties: baggageclaim.VolumeProperties{
					"property-name": "property-val",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=property-val", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(1))

			err = json.NewEncoder(body).Encode(baggageclaim.PropertyRequest{
				Value: "other-val",
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/properties/property-name", volumes[0].Handle), body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
			Expect(recorder.Body.String()).To(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes?property-name=other-val", nil)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())

			Expect(volumes).To(HaveLen(1))
		})

		It("can have its ttl updated", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				TTLInSeconds: 1,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			var firstVolume volume.Volume
			err = json.NewDecoder(recorder.Body).Decode(&firstVolume)
			Expect(err).NotTo(HaveOccurred())
			Expect(firstVolume.TTL).To(Equal(volume.TTL(1)))

			err = json.NewEncoder(body).Encode(baggageclaim.TTLRequest{
				Value: 2,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("PUT", fmt.Sprintf("/volumes/%s/ttl", firstVolume.Handle), body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
			Expect(recorder.Body.String()).To(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(1))
			Expect(volumes[0].TTL).To(Equal(volume.TTL(2)))
			Expect(volumes[0].ExpiresAt).NotTo(Equal(firstVolume.ExpiresAt))
		})
	})

	Describe("destroying a volume", func() {
		It("can be destroyed", func() {
			body := &bytes.Buffer{}

			err := json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
				Handle: "some-handle",
				Strategy: encStrategy(map[string]string{
					"type": "empty",
				}),
				TTLInSeconds: 0,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder := httptest.NewRecorder()
			request, _ := http.NewRequest("POST", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(201))

			var createdVolume volume.Volume
			err = json.NewDecoder(recorder.Body).Decode(&createdVolume)
			Expect(err).NotTo(HaveOccurred())

			err = json.NewEncoder(body).Encode(baggageclaim.TTLRequest{
				Value: 2,
			})
			Expect(err).NotTo(HaveOccurred())

			recorder = httptest.NewRecorder()
			request, err = http.NewRequest("DELETE", fmt.Sprintf("/volumes/%s", createdVolume.Handle), nil)
			Expect(err).NotTo(HaveOccurred())
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(http.StatusNoContent))
			Expect(recorder.Body.String()).To(BeEmpty())

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("GET", "/volumes", body)
			handler.ServeHTTP(recorder, request)
			Expect(recorder.Code).To(Equal(200))

			var volumes volume.Volumes
			err = json.NewDecoder(recorder.Body).Decode(&volumes)
			Expect(err).NotTo(HaveOccurred())
			Expect(volumes).To(HaveLen(0))
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

		Context("when there are properties given", func() {
			var properties baggageclaim.VolumeProperties

			Context("with valid properties", func() {
				BeforeEach(func() {
					properties = baggageclaim.VolumeProperties{
						"property-name": "property-value",
					}

					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "empty",
						}),
						Properties: properties,
					})
				})

				It("creates the properties file", func() {
					var response volume.Volume
					err := json.NewDecoder(recorder.Body).Decode(&response)
					Expect(err).NotTo(HaveOccurred())

					propertiesPath := filepath.Join(volumeDir, "live", response.Handle, "properties.json")
					Expect(propertiesPath).To(BeAnExistingFile())

					propertiesContents, err := ioutil.ReadFile(propertiesPath)
					Expect(err).NotTo(HaveOccurred())

					var storedProperties baggageclaim.VolumeProperties
					err = json.Unmarshal(propertiesContents, &storedProperties)
					Expect(err).NotTo(HaveOccurred())

					Expect(storedProperties).To(Equal(properties))
				})

				It("returns the properties in the response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"property-name":"property-value"`))
				})
			})
		})

		Context("when there are no properties given", func() {
			BeforeEach(func() {
				body = &bytes.Buffer{}
				json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
					Handle: "some-handle",
					Strategy: encStrategy(map[string]string{
						"type": "empty",
					}),
				})
			})

			It("writes a nice JSON", func() {
				Expect(recorder.Body).To(ContainSubstring(`"path":`))
				Expect(recorder.Body).To(ContainSubstring(`"handle":"some-handle"`))
			})

			Context("when handle is not provided in request", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Strategy: encStrategy(map[string]string{
							"type": "empty",
						}),
					})
				})

				It("generates a handle", func() {
					Expect(recorder.Body).To(ContainSubstring(`"path":`))
					uuidV4Regexp := `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`
					Expect(recorder.Body).To(MatchRegexp(`"handle":"` + uuidV4Regexp + `"`))
				})
			})

			Context("when invalid JSON is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{{{{{{")
				})

				It("returns a 400 Bad Request response", func() {
					Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when no strategy is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{}")
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when an unrecognized strategy is submitted", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "grime",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow but not parent volume is provided", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "cow",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow and the parent volume does not exist", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Strategy: encStrategy(map[string]string{
							"type":   "cow",
							"volume": "#pain",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getRecorder := httptest.NewRecorder()
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					handler.ServeHTTP(getRecorder, getReq)
					Expect(getRecorder.Body).To(MatchJSON("[]"))
				})
			})
		})

		Describe("creating a volume asynchronously", func() {
			var (
				recorder *httptest.ResponseRecorder
				body     io.ReadWriter
			)

			JustBeforeEach(func() {
				recorder = httptest.NewRecorder()
				request, _ := http.NewRequest("POST", "/volumes-async", body)

				handler.ServeHTTP(recorder, request)
			})

			Context("when invalid JSON is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{{{{{{")
				})

				It("returns a 400 Bad Request response", func() {
					Expect(recorder.Code).To(Equal(http.StatusBadRequest))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					Consistently(func() io.Reader {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getReq)
						return getRecorder.Body
					}).Should(MatchJSON("[]"))
				})
			})

			Context("when no strategy is submitted", func() {
				BeforeEach(func() {
					body = bytes.NewBufferString("{}")
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					Consistently(func() io.Reader {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getReq)
						return getRecorder.Body
					}).Should(MatchJSON("[]"))
				})
			})

			Context("when an unrecognized strategy is submitted", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "grime",
						}),
					})
				})

				It("returns a 422 Unprocessable Entity response", func() {
					Expect(recorder.Code).To(Equal(422))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"error":`))
				})

				It("does not create a volume", func() {
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					Consistently(func() io.Reader {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getReq)
						return getRecorder.Body
					}).Should(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow but no parent volume is provided", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type": "cow",
						}),
					})
				})

				It("returns a 201 Created response", func() {
					Expect(recorder.Code).To(Equal(201))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"handle":"some-handle"`))
				})

				It("does not create a volume", func() {
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					Consistently(func() io.Reader {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getReq)
						return getRecorder.Body
					}).Should(MatchJSON("[]"))
				})
			})

			Context("when the strategy is cow and the parent volume does not exist", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Handle: "some-handle",
						Strategy: encStrategy(map[string]string{
							"type":   "cow",
							"volume": "#pain",
						}),
					})
				})

				It("returns a 201 Created response", func() {
					Expect(recorder.Code).To(Equal(201))
				})

				It("writes a nice JSON response", func() {
					Expect(recorder.Body).To(ContainSubstring(`"handle":"some-handle"`))
				})

				It("does not create a volume", func() {
					getReq, _ := http.NewRequest("GET", "/volumes", nil)
					Consistently(func() io.Reader {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getReq)
						return getRecorder.Body
					}).Should(MatchJSON("[]"))
				})
			})

			Context("when handle is not provided in request", func() {
				BeforeEach(func() {
					body = &bytes.Buffer{}
					json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
						Strategy: encStrategy(map[string]string{
							"type": "empty",
						}),
					})
				})

				It("generates a handle", func() {
					uuidV4Regexp := `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[0-9a-f]{4}-[0-9a-f]{12}`

					var response baggageclaim.VolumeFutureResponse
					err := json.NewDecoder(recorder.Body).Decode(&response)
					Expect(err).NotTo(HaveOccurred())
					Expect(response.Handle).To(MatchRegexp(uuidV4Regexp))

					getRequest, _ := http.NewRequest("GET", "/volumes-async/"+response.Handle, nil)
					Eventually(func() int {
						getRecorder := httptest.NewRecorder()
						handler.ServeHTTP(getRecorder, getRequest)
						return getRecorder.Code
					}).Should(Equal(http.StatusOK))
				})
			})

			Context("when there are properties given", func() {
				var properties baggageclaim.VolumeProperties

				Context("with valid properties", func() {
					BeforeEach(func() {
						properties = baggageclaim.VolumeProperties{
							"property-name": "property-value",
						}

						body = &bytes.Buffer{}
						json.NewEncoder(body).Encode(baggageclaim.VolumeRequest{
							Handle: "some-handle",
							Strategy: encStrategy(map[string]string{
								"type": "empty",
							}),
							Properties: properties,
						})
					})

					It("returns a handle", func() {
						var response baggageclaim.VolumeFutureResponse
						err := json.NewDecoder(recorder.Body).Decode(&response)
						Expect(err).NotTo(HaveOccurred())
						Expect(response.Handle).To(Equal("some-handle"))

						request, _ := http.NewRequest("GET", "/volumes-async/"+response.Handle, nil)
						Eventually(func() int {
							recorder = httptest.NewRecorder()
							handler.ServeHTTP(recorder, request)
							return recorder.Code
						}).Should(Equal(http.StatusOK))
					})
				})
			})
		})
	})
})

func encStrategy(strategy map[string]string) *json.RawMessage {
	bytes, err := json.Marshal(strategy)
	Expect(err).NotTo(HaveOccurred())

	msg := json.RawMessage(bytes)

	return &msg
}
