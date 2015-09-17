package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim"
)

var _ = Describe("Restarting", func() {
	var (
		runner *BaggageClaimRunner
		client baggageclaim.Client
	)

	BeforeEach(func() {
		runner = NewRunner(baggageClaimPath)
		runner.Start()

		client = runner.Client()
	})

	AfterEach(func() {
		runner.Stop()
		runner.Cleanup()
	})

	It("can get volumes after the process restarts", func() {
		createdVolume, err := client.CreateEmptyVolume(baggageclaim.VolumeSpec{})
		Ω(err).ShouldNot(HaveOccurred())

		Ω(runner.CurrentHandles()).Should(ConsistOf(createdVolume.Handle()))

		runner.Bounce()

		Ω(runner.CurrentHandles()).Should(ConsistOf(createdVolume.Handle()))
	})
})
