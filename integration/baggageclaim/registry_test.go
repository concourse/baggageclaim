package integration_test

import (
	"github.com/concourse/baggageclaim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("baggageclaim registry", func() {

	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client
	)

	BeforeEach(func() {
		runner = NewRunner(baggageClaimPath, "naive")
		runner.Start()

		client = runner.Client()
	})

	AfterEach(func() {
		runner.Stop()
		runner.Cleanup()
	})

	Context("success", func() {
		var vol baggageclaim.Volume

		BeforeEach(func() {
			var err error

			vol, err = client.CreateVolume(logger, "handle", baggageclaim.VolumeSpec{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("hdsuh", func() {
			runner
		})

		AfterEach(func() {
			client.DestroyVolume(logger, "handle")
		})
	})

	// 1. create an empty volume
	// 2. create a fixture path where we put an image tarball
	// 3. import that
	// 3. try to pull from that volume
	//

})

// pullImage attempts to pull a container image from a registry.
//
func pullImage(address string) error {

}
