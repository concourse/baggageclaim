package baggageclaim_test

import (
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
	Describe("Interacting with the server", func() {
		var (
			bcServer *ghttp.Server
			logger   lager.Logger
			bcClient baggageclaim.Client
		)

		BeforeEach(func() {
			bcServer = ghttp.NewServer()
			logger = lagertest.NewTestLogger("client")
			bcClient = client.New(bcServer.URL())
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
			It("returns volume if it exists", func() {
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
				)
				volume, found, err := bcClient.LookupVolume(logger, "some-handle")
				Expect(volume.Handle()).To(Equal(expectedVolume.Handle))
				Expect(found).To(BeTrue())
				Expect(err).ToNot(HaveOccurred())
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
			It("it returns list of volumes", func() {
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
				)
				volumes, err := bcClient.ListVolumes(logger, baggageclaim.VolumeProperties{})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(2))
				Expect(volumes[0].Handle()).To(Equal("some-handle"))
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

		Describe("Creating volumes", func() {
			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("POST", "/volumes", "lost baggage", http.StatusInternalServerError)
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
						ghttp.VerifyRequest("POST", "/volumes"),
						ghttp.RespondWithJSONEncoded(201, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
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
				err := vol.StreamIn(".", strings.NewReader("some tar content"))
				Expect(err).ToNot(HaveOccurred())

				Expect(bodyChan).To(Receive(Equal([]byte("some tar content"))))
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-in", "lost baggage", http.StatusInternalServerError)
					err := vol.StreamIn("./some/path/", strings.NewReader("even more tar"))
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
						ghttp.VerifyRequest("POST", "/volumes"),
						ghttp.RespondWithJSONEncoded(201, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
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
				out, err := vol.StreamOut(".")
				Expect(err).NotTo(HaveOccurred())

				b, err := ioutil.ReadAll(out)
				Expect(err).NotTo(HaveOccurred())

				Expect(string(b)).To(Equal("some tar content"))
			})

			Context("when error occurs", func() {
				It("returns API error message", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusInternalServerError)
					_, err := vol.StreamOut("./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})

				It("returns ErrVolumeNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", "lost baggage", http.StatusNotFound)
					_, err := vol.StreamOut("./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
				})

				It("returns ErrFileNotFound", func() {
					mockErrorResponse("PUT", "/volumes/some-handle/stream-out", api.ErrStreamOutNotFound.Error(), http.StatusNotFound)
					_, err := vol.StreamOut("./some/path/")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(baggageclaim.ErrFileNotFound))
				})
			})
		})

		Describe("Setting property on a volume", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes"),
						ghttp.RespondWithJSONEncoded(201, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
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

		Describe("Getting volume stats", func() {
			var vol baggageclaim.Volume
			BeforeEach(func() {
				bcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/volumes"),
						ghttp.RespondWithJSONEncoded(201, volume.Volume{
							Handle:     "some-handle",
							Path:       "some-path",
							Properties: volume.Properties{},
							TTL:        volume.TTL(1),
							ExpiresAt:  time.Now().Add(time.Second),
						}),
					),
				)
				var err error
				vol, err = bcClient.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{})
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when unexpected error occurs", func() {
				It("returns error code and useful message", func() {
					mockErrorResponse("GET", "/volumes/some-handle/stats", "lost baggage", http.StatusInternalServerError)
					_, err := vol.SizeInBytes()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("lost baggage"))
				})
			})
		})
	})
})
