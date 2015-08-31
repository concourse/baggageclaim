package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/integration/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Restarting", func() {
	var (
		runner *BaggageClaimRunner
		client *integration.Client
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
		createdVolume, err := client.CreateEmptyVolume(volume.Properties{})
		Ω(err).ShouldNot(HaveOccurred())

		volumes, err := client.GetVolumes()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(volumes).Should(ConsistOf(createdVolume))

		runner.Bounce()

		volumesAfterRestart, err := client.GetVolumes()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(volumesAfterRestart).Should(ConsistOf(createdVolume))
	})
})
