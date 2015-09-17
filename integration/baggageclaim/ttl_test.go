package integration_test

import (
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TTL's", func() {
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

	It("can set a ttl", func() {
		spec := baggageclaim.VolumeSpec{
			TTLInSeconds: 10,
		}
		emptyVolume, err := client.CreateEmptyVolume(spec)
		Ω(err).ShouldNot(HaveOccurred())

		expiresAt := time.Now().Add(volume.TTL(10).Duration())

		someVolume, err := client.GetVolume(emptyVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.TTL()).Should(Equal(uint(10)))
		Ω(someVolume.ExpiresAt()).Should(BeTemporally("~", expiresAt, 1*time.Second))
	})

	It("removes the volume after the ttl duration", func() {
		spec := baggageclaim.VolumeSpec{
			TTLInSeconds: 1,
		}
		emptyVolume, err := client.CreateEmptyVolume(spec)
		Ω(err).ShouldNot(HaveOccurred())

		volumes, err := client.GetVolumes()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(volumes).Should(HaveLen(1))

		Eventually(runner.CurrentHandles, 2*time.Second).ShouldNot(ContainElement(emptyVolume.Handle()))
	})

	Context("resetting the ttl", func() {
		It("pauses the parent if you create a cow volume", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 2,
			}
			parentVolume, err := client.CreateEmptyVolume(spec)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume, err := client.CreateCOWVolume(baggageclaim.VolumeSpec{
				ParentHandle: parentVolume.Handle(),
				TTLInSeconds: 4,
			})
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(parentVolume.Handle()))
			Eventually(runner.CurrentHandles, 2*time.Second).ShouldNot(ContainElement(childVolume.Handle()))

			Eventually(runner.CurrentHandles, 3*time.Second).ShouldNot(ContainElement(parentVolume.Handle()))
		})

		It("pauses the parent as long as *any* child volumes are present", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 2,
			}
			parentVolume, err := client.CreateEmptyVolume(spec)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume1, err := client.CreateCOWVolume(baggageclaim.VolumeSpec{
				ParentHandle: parentVolume.Handle(),
				TTLInSeconds: 4,
			})
			Ω(err).ShouldNot(HaveOccurred())

			childVolume2, err := client.CreateCOWVolume(baggageclaim.VolumeSpec{
				ParentHandle: parentVolume.Handle(),
				TTLInSeconds: 9,
			})
			Ω(err).ShouldNot(HaveOccurred())

			By("the parent should stay paused")
			Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(parentVolume.Handle()))

			By("the first child should be removed")
			Eventually(runner.CurrentHandles, 2*time.Second).ShouldNot(ContainElement(childVolume1.Handle()))

			By("the parent should still be paused")
			Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(parentVolume.Handle()))

			By("the second child should be removed")
			Eventually(runner.CurrentHandles, 3*time.Second).ShouldNot(ContainElement(childVolume2.Handle()))

			By("the parent should be removed")
			Eventually(runner.CurrentHandles, 3*time.Second).ShouldNot(ContainElement(parentVolume.Handle()))
		})

		It("resets to a new value if you update the ttl", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 1,
			}
			emptyVolume, err := client.CreateEmptyVolume(spec)
			Ω(err).ShouldNot(HaveOccurred())

			err = client.SetTTL(emptyVolume.Handle(), 3)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 2*time.Second).Should(ContainElement(emptyVolume.Handle()))
			Eventually(runner.CurrentHandles, 2*time.Second).ShouldNot(ContainElement(emptyVolume.Handle()))
		})
	})
})
