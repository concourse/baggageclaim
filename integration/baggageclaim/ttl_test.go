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

		emptyVolume, err := client.CreateVolume(logger, spec)
		Ω(err).ShouldNot(HaveOccurred())

		expiresAt := time.Now().Add(volume.TTL(10).Duration())

		someVolume, err := client.LookupVolume(logger, emptyVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		ttl, expiresAt, err := someVolume.Expiration()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(ttl).Should(Equal(uint(10)))
		Ω(expiresAt).Should(BeTemporally("~", expiresAt, 1*time.Second))
	})

	It("removes the volume after the ttl duration", func() {
		spec := baggageclaim.VolumeSpec{
			TTLInSeconds: 1,
		}

		emptyVolume, err := client.CreateVolume(logger, spec)
		Ω(err).ShouldNot(HaveOccurred())

		emptyVolume.Release()

		volumes, err := client.ListVolumes(logger, nil)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(volumes).Should(HaveLen(1))
		volumes[0].Release()

		time.Sleep(2 * time.Second)

		Expect(runner.CurrentHandles()).To(BeEmpty())
	})

	Describe("heartbeating", func() {
		It("keeps the container alive, and lets it expire once released", func() {
			spec := baggageclaim.VolumeSpec{TTLInSeconds: 2}

			volume, err := client.CreateVolume(logger, spec)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(volume.Handle()))

			volume.Release()

			// note: don't use Eventually; CurrentHandles causes it to heartbeat

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(BeEmpty())
		})
	})

	Describe("resetting the ttl", func() {
		It("pauses the parent if you create a cow volume", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 2,
			}

			parentVolume, err := client.CreateVolume(logger, spec)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
				Strategy:     baggageclaim.COWStrategy{Parent: parentVolume},
				TTLInSeconds: 4,
			})
			Ω(err).ShouldNot(HaveOccurred())

			parentVolume.Release()

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			childVolume.Release()

			time.Sleep(5 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume.Handle()))

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(parentVolume.Handle()))
		})

		It("pauses the parent as long as *any* child volumes are present", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 2,
			}
			parentVolume, err := client.CreateVolume(logger, spec)
			Ω(err).ShouldNot(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume1, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
				Strategy:     baggageclaim.COWStrategy{Parent: parentVolume},
				TTLInSeconds: 2,
			})
			Ω(err).ShouldNot(HaveOccurred())

			childVolume2, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
				Strategy:     baggageclaim.COWStrategy{Parent: parentVolume},
				TTLInSeconds: 2,
			})
			Ω(err).ShouldNot(HaveOccurred())

			parentVolume.Release()

			By("the parent should stay paused")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			By("the first child should be removed")
			childVolume1.Release()
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume1.Handle()))

			By("the parent should still be paused")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			By("the second child should be removed")
			childVolume2.Release()
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume2.Handle()))

			By("the parent should be removed")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(parentVolume.Handle()))
		})

		It("resets to a new value if you update the ttl", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 2,
			}

			emptyVolume, err := client.CreateVolume(logger, spec)
			Ω(err).ShouldNot(HaveOccurred())

			ttl, _, err := emptyVolume.Expiration()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(ttl).Should(Equal(uint(2)))

			emptyVolume.Release()

			err = emptyVolume.SetTTL(3)
			Ω(err).ShouldNot(HaveOccurred())

			ttl, _, err = emptyVolume.Expiration()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(ttl).Should(Equal(uint(3)))
		})

		It("returns ErrVolumeNotFound when setting the TTL after it's expired", func() {
			spec := baggageclaim.VolumeSpec{
				TTLInSeconds: 1,
			}

			emptyVolume, err := client.CreateVolume(logger, spec)
			Ω(err).ShouldNot(HaveOccurred())

			emptyVolume.Release()
			time.Sleep(2 * time.Second)

			err = emptyVolume.SetTTL(1)
			Ω(err).Should(Equal(baggageclaim.ErrVolumeNotFound))
		})
	})
})
