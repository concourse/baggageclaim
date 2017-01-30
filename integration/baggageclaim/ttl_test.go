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
			TTL: 10 * time.Second,
		}

		emptyVolume, err := client.CreateVolume(logger, "some-handle", spec)
		Expect(err).NotTo(HaveOccurred())

		expectedExpiresAt := time.Now().Add(volume.TTL(10).Duration())

		someVolume, found, err := client.LookupVolume(logger, emptyVolume.Handle())
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		ttl, actualExpiresAt, err := someVolume.Expiration()
		Expect(err).NotTo(HaveOccurred())
		Expect(ttl).To(Equal(10 * time.Second))
		Expect(actualExpiresAt).To(BeTemporally("~", expectedExpiresAt, 1*time.Second))
	})

	It("removes the volume after the ttl duration", func() {
		spec := baggageclaim.VolumeSpec{
			TTL: 1 * time.Second,
		}

		emptyVolume, err := client.CreateVolume(logger, "some-handle", spec)
		Expect(err).NotTo(HaveOccurred())

		emptyVolume.Release(nil)

		volumes, err := client.ListVolumes(logger, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(volumes).To(HaveLen(1))

		volumes[0].Release(nil)

		time.Sleep(2 * time.Second)

		Expect(runner.CurrentHandles()).To(BeEmpty())
	})

	Describe("heartbeating", func() {
		It("keeps the container alive, and lets it expire once released", func() {
			spec := baggageclaim.VolumeSpec{TTL: 2 * time.Second}

			volume, err := client.CreateVolume(logger, "some-handle", spec)
			Expect(err).NotTo(HaveOccurred())

			Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(volume.Handle()))

			volume.Release(nil)

			// note: don't use Eventually; CurrentHandles causes it to heartbeat

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(BeEmpty())
		})

		Describe("releasing with a final ttl", func() {
			It("lets it expire after the given TTL", func() {
				spec := baggageclaim.VolumeSpec{TTL: 2 * time.Second}

				volume, err := client.CreateVolume(logger, "some-handle", spec)
				Expect(err).NotTo(HaveOccurred())

				Consistently(runner.CurrentHandles, 3*time.Second).Should(ContainElement(volume.Handle()))

				volume.Release(baggageclaim.FinalTTL(3 * time.Second))

				ttl, _, err := volume.Expiration()
				Expect(err).NotTo(HaveOccurred())
				Expect(ttl).To(Equal(3 * time.Second))

				time.Sleep(4 * time.Second)
				Expect(runner.CurrentHandles()).To(BeEmpty())
			})
		})

		Context("when you look up a volume by handle", func() {
			It("heartbeats the volume once before returning it", func() {
				spec := baggageclaim.VolumeSpec{
					TTL: 5 * time.Second,
				}

				emptyVolume, err := client.CreateVolume(logger, "some-handle", spec)
				Ω(err).ShouldNot(HaveOccurred())

				time.Sleep(2 * time.Second)

				lookedUpAt := time.Now()

				_, found, err := client.LookupVolume(logger, emptyVolume.Handle())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(found).To(BeTrue())

				_, expiresAt, err := emptyVolume.Expiration()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(expiresAt).Should(BeTemporally("~", lookedUpAt.Add(5*time.Second), 1*time.Second))
			})
		})
	})

	Describe("resetting the ttl", func() {
		It("pauses the parent if you create a cow volume", func() {
			spec := baggageclaim.VolumeSpec{
				TTL: 2 * time.Second,
			}

			parentVolume, err := client.CreateVolume(logger, "parent-handle", spec)
			Expect(err).NotTo(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume, err := client.CreateVolume(logger, "cow-handle", baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{Parent: parentVolume},
				TTL:      4 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			parentVolume.Release(nil)

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			childVolume.Release(nil)

			time.Sleep(5 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume.Handle()))

			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(parentVolume.Handle()))
		})

		It("pauses the parent as long as *any* child volumes are present", func() {
			spec := baggageclaim.VolumeSpec{
				TTL: 2 * time.Second,
			}
			parentVolume, err := client.CreateVolume(logger, "parent-handle", spec)
			Expect(err).NotTo(HaveOccurred())

			Consistently(runner.CurrentHandles, 1*time.Second).Should(ContainElement(parentVolume.Handle()))

			childVolume1, err := client.CreateVolume(logger, "child-handle-1", baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{Parent: parentVolume},
				TTL:      2 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			childVolume2, err := client.CreateVolume(logger, "child-handle-2", baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{Parent: parentVolume},
				TTL:      2 * time.Second,
			})
			Expect(err).NotTo(HaveOccurred())

			parentVolume.Release(nil)

			By("the parent should stay paused")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			By("the first child should be removed")
			childVolume1.Release(nil)
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume1.Handle()))

			By("the parent should still be paused")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).To(ContainElement(parentVolume.Handle()))

			By("the second child should be removed")
			childVolume2.Release(nil)
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(childVolume2.Handle()))

			By("the parent should be removed")
			time.Sleep(3 * time.Second)
			Expect(runner.CurrentHandles()).ToNot(ContainElement(parentVolume.Handle()))
		})

		It("resets to a new value if you update the ttl", func() {
			spec := baggageclaim.VolumeSpec{
				TTL: 2 * time.Second,
			}

			emptyVolume, err := client.CreateVolume(logger, "some-handle", spec)
			Expect(err).NotTo(HaveOccurred())

			ttl, _, err := emptyVolume.Expiration()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(2 * time.Second))

			emptyVolume.Release(nil)

			err = emptyVolume.SetTTL(3 * time.Second)
			Expect(err).NotTo(HaveOccurred())

			ttl, _, err = emptyVolume.Expiration()
			Expect(err).NotTo(HaveOccurred())
			Expect(ttl).To(Equal(3 * time.Second))
		})

		It("returns ErrVolumeNotFound when setting the TTL after it's expired", func() {
			spec := baggageclaim.VolumeSpec{
				TTL: 1 * time.Second,
			}

			emptyVolume, err := client.CreateVolume(logger, "some-handle", spec)
			Expect(err).NotTo(HaveOccurred())

			emptyVolume.Release(nil)
			time.Sleep(2 * time.Second)

			err = emptyVolume.SetTTL(1 * time.Second)
			Expect(err).To(Equal(baggageclaim.ErrVolumeNotFound))
		})
	})
})
