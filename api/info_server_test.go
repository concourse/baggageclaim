package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/api"
	"github.com/concourse/baggageclaim/uidgid"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("InfoServer", func() {
	var (
		handler  http.Handler
		recorder *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("volume-server")
		namespacer := &uidgid.NoopNamespacer{}
		strategerizer := volume.NewStrategerizer(namespacer)
		fs, err := volume.NewFilesystem(&driver.NaiveDriver{}, "some-volume-dir")
		Expect(err).NotTo(HaveOccurred())
		repo := volume.NewRepository(logger, fs, volume.NewLockManager())
		handler, err = api.NewHandler(logger, strategerizer, namespacer, repo)
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		recorder = httptest.NewRecorder()
		request, _ := http.NewRequest("GET", "/info", nil)

		handler.ServeHTTP(recorder, request)
	})

	It("returns protocol version", func() {
		Expect(recorder.Body).To(MatchJSON(fmt.Sprintf(`{"protocol_version": %d}`, baggageclaim.ProtocolVersion)))
	})
})
