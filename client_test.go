package baggageclaim_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/client"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Baggage Claim Client", func() {
	Describe("getting the heartbeat interval from a TTL", func() {
		It("has an upper bound of 1 minute", func() {
			interval := client.IntervalForTTL(500 * time.Second)

			Expect(interval).To(Equal(time.Minute))
		})

		Context("when the TTL is small", func() {
			It("returns an interval that is half of the TTL", func() {
				interval := client.IntervalForTTL(5 * time.Second)

				Expect(interval).To(Equal(2500 * time.Millisecond))
			})
		})

		Context("when the TTL is zero", func() {
			It("keeps the TTL at zero", func() {
				interval := client.IntervalForTTL(0 * time.Second)

				Expect(interval).To(Equal(0 * time.Second))
			})
		})
	})

	Describe("Interacting with the server", func() {
		var (
			bcServer *ghttp.Server
			logger   lager.Logger
			bcClient baggageclaim.Client
		)

		BeforeEach(func() {
			bcServer = ghttp.NewServer()
			logger = lagertest.NewTestLogger("client")
			bcClient = client.New(bcServer.URL(), &http.Transport{DisableKeepAlives: true})
		})

		AfterEach(func() {
			bcServer.Close()
		})

		mockErrorResponse := func(method string, endpoint string, message string, status int) {
			response := fmt.Sprintf(`{"error":"%s"}`, message)
			bcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest(method, endpoint),
					ghttp.RespondWith(status, response),
				),
			)
		}

		Describe("Looking up a volume by handle", func() {
			It("heartbeats immediately to reset the TTL", func() {
				didHeartbeat := make(chan struct{})

				expectedVolume := volume.Volume{
					Handle:     "some-handle",
					Path:       "some-path",
					Properties: volume.Properties{},
					TTL:        volume.TTL(1),
					ExpiresAt:  time.Now().Add(time.Second),
				}

				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedVolume),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
						func(w http.ResponseWriter, r *http.Request) {
							close(didHeartbeat)
						},
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				volume, found, err := bcClient.LookupVolume(logger, "some-handle")
				Expect(volume.Handle()).To(Equal(expectedVolume.Handle))
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())

				Eventually(didHeartbeat, time.Second).Should(BeClosed())
			})

			Context("when the volume's TTL is 0", func() {
				It("does not heartbeat, and allows the volume to be released", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes/some-handle"),
							ghttp.RespondWithJSONEncoded(200, volume.Volume{
								Handle:     "some-handle",
								Path:       "some-path",
								Properties: volume.Properties{},
								TTL:        volume.TTL(0),
								ExpiresAt:  time.Now().Add(time.Second),
							}),
						),
					)

					volume, _, err := bcClient.LookupVolume(logger, "some-handle")
					Expect(err).NotTo(HaveOccurred())

					Consistently(bcServer.ReceivedRequests()).Should(HaveLen(1))

					volume.Release(baggageclaim.FinalTTL(5 * time.Second))
					Expect(bcServer.ReceivedRequests()).To(HaveLen(1))
				})
			})

			Context("when the intial heartbeat fails", func() {
				It("reports that the volume could not be found", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes/some-handle"),
							ghttp.RespondWithJSONEncoded(200, volume.Volume{
								Handle:     "some-handle",
								Path:       "some-path",
								Properties: volume.Properties{},
								TTL:        volume.TTL(1),
								ExpiresAt:  time.Now().Add(time.Second),
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
							func(w http.ResponseWriter, r *http.Request) {
								api.RespondWithError(w, volume.ErrVolumeDoesNotExist, http.StatusNotFound)
							},
						),
					)
					foundVolume, found, err := bcClient.LookupVolume(logger, "some-handle")
					Expect(foundVolume).To(BeNil())
					Expect(found).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the volume does not exist", func() {
				It("reports that the volume could not be found", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes/some-handle"),
							ghttp.RespondWith(http.StatusNotFound, ""),
						),
					)
					foundVolume, found, err := bcClient.LookupVolume(logger, "some-handle")
					Expect(foundVolume).To(BeNil())
					Expect(found).To(BeFalse())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("GET", "/volumes/some-handle", "lost baggage", http.StatusInternalServerError)
					foundVolume, found, err := bcClient.LookupVolume(logger, "some-handle")
					Expect(foundVolume).To(BeNil())
					Expect(found).To(BeFalse())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Listing volumes", func() {
			Context("when the inital heartbeat fails for a volume", func() {
				It("it is omitted from the returned list of volumes", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes"),
							ghttp.RespondWithJSONEncoded(200, []volume.Volume{
								{
									Handle:     "some-handle",
									Path:       "some-path",
									Properties: volume.Properties{},
									TTL:        volume.TTL(1),
									ExpiresAt:  time.Now().Add(time.Second),
								},
								{
									Handle:     "another-handle",
									Path:       "some-path",
									Properties: volume.Properties{},
									TTL:        volume.TTL(1),
									ExpiresAt:  time.Now().Add(time.Second),
								},
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
							func(w http.ResponseWriter, r *http.Request) {
								w.WriteHeader(http.StatusNoContent)
							},
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/volumes/another-handle/ttl"),
							func(w http.ResponseWriter, r *http.Request) {
								api.RespondWithError(w, volume.ErrVolumeDoesNotExist, http.StatusNotFound)
							},
						),
					)
					volumes, err := bcClient.ListVolumes(logger, baggageclaim.VolumeProperties{})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(1))
					Expect(volumes[0].Handle()).To(Equal("some-handle"))
				})
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("GET", "/volumes", "lost baggage", http.StatusInternalServerError)
					volumes, err := bcClient.ListVolumes(logger, baggageclaim.VolumeProperties{})
					Expect(volumes).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Destroying volumes", func() {
			Context("when all volumes are destroyed as requested", func() {
				var handles = []string{"some-handle"}
				var buf bytes.Buffer
				json.NewEncoder(&buf).Encode(handles)

				It("it returns all handles in response", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.VerifyBody(buf.Bytes()),
							ghttp.RespondWithJSONEncoded(204, nil),
						))

					err := bcClient.DestroyVolumes(logger, handles)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("when no volumes are destroyes", func() {
				var handles = []string{"some-handle"}
				var buf bytes.Buffer
				json.NewEncoder(&buf).Encode(handles)

				It("it returns no handles", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes/destroy"),
							ghttp.VerifyBody(buf.Bytes()),
							ghttp.RespondWithJSONEncoded(500, handles),
						))

					err := bcClient.DestroyVolumes(logger, handles)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Describe("Creating volumes", func() {
			Context("when the inital heartbeat fails for the volume", func() {
				It("reports that the volume could not be found", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/volumes-async"),
							ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
								Handle: "some-handle",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
								Handle:     "some-handle",
								Path:       "some-path",
								Properties: volume.Properties{},
								TTL:        volume.TTL(1),
								ExpiresAt:  time.Now().Add(time.Second),
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
							func(w http.ResponseWriter, r *http.Request) {
								api.RespondWithError(w, volume.ErrVolumeDoesNotExist, http.StatusNotFound)
							},
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)
					createdVolume, err := bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
					Expect(createdVolume).To(BeNil())
					Expect(err).To(Equal(volume.ErrVolumeDoesNotExist))
				})
			})

			Context("when the TTL is 0", func() {
				It("does not call set TTL", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/volumes-async"),
							ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
								Handle: "some-handle",
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
								Handle:     "some-handle",
								Path:       "some-path",
								Properties: volume.Properties{},
								TTL:        volume.TTL(0),
								ExpiresAt:  time.Now().Add(time.Second),
							}),
						),
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)

					_, err := bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
					Expect(err).To(Not(HaveOccurred()))

					Consistently(bcServer.ReceivedRequests()).Should(HaveLen(3))
				})
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/volumes-async"),
							ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
								Handle: "some-handle",
							}),
						),
					)
					mockErrorResponse("GET", "/volumes-async/some-handle", "lost baggage", http.StatusInternalServerError)
					bcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
							ghttp.RespondWith(http.StatusNoContent, ""),
						),
					)

					createdVolume, err := bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
					Expect(createdVolume).To(BeNil())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Stream in a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("streams the volume", func() {
				bodyChan := make(chan []byte, 1)

				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/stream-in"),
						func(w http.ResponseWriter, r *http.Request) {
							str, _ := ioutil.ReadAll(r.Body)
							bodyChan <- str
						},
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				err := vol.StreamIn(context.TODO(), ".", strings.NewReader("some tar content"))
				Expect(err).ToNot(HaveOccurred())

				Expect(bodyChan).To(Receive(Equal([]byte("some tar content"))))
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-in", "lost baggage", http.StatusInternalServerError)
					err := vol.StreamIn(context.TODO(), "./some/path/", strings.NewReader("even more tar"))
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})

		Describe("Stream out a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("streams the volume", func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/stream-out"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Write([]byte("some tar content"))
						},
					),
				)
				out, err := vol.StreamOut(context.TODO(), ".")
				Expect(err).NotTo(HaveOccurred())

				b, err := ioutil.ReadAll(out)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(b)).To(Equal("some tar content"))
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusInternalServerError)
					_, err := vol.StreamOut(context.TODO(), "./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusNotFound)
					_, err := vol.StreamOut(context.TODO(), "./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})

				It("returns ErrFileNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", api.ErrStreamOutNotFound.Error(), http.StatusNotFound)
					_, err := vol.StreamOut(context.TODO(), "./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrFileNotFound))
				})
			})
		})

		Describe("Setting TTL on a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/ttl", "lost baggage", http.StatusInternalServerError)
					err := vol.SetTTL(time.Second)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/ttl", "lost baggage", http.StatusNotFound)
					err := vol.SetTTL(time.Second)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})
			})
		})

		Describe("Setting property on a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes-async"),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, baggageclaim.VolumeFutureResponse{
							Handle: "some-handle",
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/volumes-async/some-handle"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/volumes/some-handle/ttl"),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/volumes-async/some-handle"),
						ghttp.RespondWith(http.StatusNoContent, nil),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/properties/key", "lost baggage", http.StatusInternalServerError)
					err := vol.SetProperty("key", "value")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/properties/key", "lost baggage", http.StatusNotFound)
					err := vol.SetProperty("key", "value")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})
			})
		})
	})
})
