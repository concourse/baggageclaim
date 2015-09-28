package integration_test

import (
	"time"

	"github.com/concourse/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
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

	It("can manage properties", func() {
		emptyVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = emptyVolume.SetProperty("another-property", "another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err := client.LookupVolume(logger, emptyVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties()).Should(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))

		err = someVolume.SetProperty("another-property", "yet-another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err = client.LookupVolume(logger, someVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties()).Should(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "yet-another-value",
		}))
	})

	It("can find a volume by its properties", func() {
		_, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{})
		Ω(err).ShouldNot(HaveOccurred())

		emptyVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
			Properties: baggageclaim.VolumeProperties{
				"property-name": "property-value",
			},
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = emptyVolume.SetProperty("another-property", "another-value")
		Ω(err).ShouldNot(HaveOccurred())

		foundVolumes, err := client.ListVolumes(logger, baggageclaim.VolumeProperties{
			"another-property": "another-value",
		})
		Ω(err).ShouldNot(HaveOccurred())

		Ω(foundVolumes).Should(HaveLen(1))
		Ω(foundVolumes[0].Properties()).Should(Equal(baggageclaim.VolumeProperties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))
	})

	It("returns ErrVolumeNotFound if the specified volume does not exist", func() {
		volume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
			TTLInSeconds: 1,
		})
		Ω(err).ShouldNot(HaveOccurred())

		volume.Release()
		time.Sleep(2 * time.Second)

		err = volume.SetProperty("some", "property")
		Ω(err).Should(Equal(baggageclaim.ErrVolumeNotFound))
	})
})
