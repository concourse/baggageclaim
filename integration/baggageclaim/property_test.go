package integration_test

import (
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
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
		emptyVolume, err := client.CreateEmptyVolume(baggageclaim.VolumeSpec{
			Properties: volume.Properties{
				"property-name": "property-value",
			},
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = client.SetProperty(emptyVolume.Handle(), "another-property", "another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err := client.GetVolume(emptyVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties()).Should(Equal(volume.Properties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))

		err = client.SetProperty(someVolume.Handle(), "another-property", "yet-another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err = client.GetVolume(someVolume.Handle())
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties()).Should(Equal(volume.Properties{
			"property-name":    "property-value",
			"another-property": "yet-another-value",
		}))
	})

	It("can find a volume by its properties", func() {
		_, err := client.CreateEmptyVolume(baggageclaim.VolumeSpec{})
		Ω(err).ShouldNot(HaveOccurred())

		emptyVolume, err := client.CreateEmptyVolume(baggageclaim.VolumeSpec{
			Properties: volume.Properties{
				"property-name": "property-value",
			},
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = client.SetProperty(emptyVolume.Handle(), "another-property", "another-value")
		Ω(err).ShouldNot(HaveOccurred())

		foundVolumes, err := client.FindVolumes(baggageclaim.VolumeProperties{
			"another-property": "another-value",
		})
		Ω(err).ShouldNot(HaveOccurred())

		Ω(foundVolumes).Should(HaveLen(1))
		Ω(foundVolumes[0].Properties()).Should(Equal(volume.Properties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))
	})

	It("returns ErrVolumeNotFound if the specified volume does not exist", func() {
		err := client.SetProperty("bogus", "some", "property")
		Ω(err).Should(Equal(baggageclaim.ErrVolumeNotFound))
	})
})
