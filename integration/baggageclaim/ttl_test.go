package integration_test

import (
	"time"

	"github.com/concourse/baggageclaim/integration/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TTL's", func() {
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

	It("can set a ttl", func() {
		spec := integration.VolumeSpec{
			TTL: 10,
		}
		emptyVolume, err := client.CreateEmptyVolume(spec)
		Ω(err).ShouldNot(HaveOccurred())

		expiresAt := time.Now().Add(volume.TTL(10).Duration())

		someVolume, err := client.GetVolume(emptyVolume.GUID)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.TTL).Should(Equal(volume.TTL(10)))
		Ω(someVolume.ExpiresAt).Should(BeTemporally("~", expiresAt, 1*time.Second))
	})

	It("removes the volume after the ttl duration", func() {
		spec := integration.VolumeSpec{
			TTL: 1,
		}
		emptyVolume, err := client.CreateEmptyVolume(spec)
		Ω(err).ShouldNot(HaveOccurred())

		volumes, err := client.GetVolumes()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(volumes).Should(HaveLen(1))

		Eventually(client.GetVolumes, 2*time.Second, 500*time.Millisecond).ShouldNot(ConsistOf(emptyVolume))
	})
})
