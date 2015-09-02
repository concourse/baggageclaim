package integration_test

import (
	"github.com/concourse/baggageclaim/integration/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Properties", func() {
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

	It("can manage properties", func() {
		emptyVolume, err := client.CreateEmptyVolume(integration.VolumeSpec{
			Properties: volume.Properties{
				"property-name": "property-value",
			},
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = client.SetProperty(emptyVolume.Handle, "another-property", "another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err := client.GetVolume(emptyVolume.Handle)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties).Should(Equal(volume.Properties{
			"property-name":    "property-value",
			"another-property": "another-value",
		}))

		err = client.SetProperty(someVolume.Handle, "another-property", "yet-another-value")
		Ω(err).ShouldNot(HaveOccurred())

		someVolume, err = client.GetVolume(someVolume.Handle)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(someVolume.Properties).Should(Equal(volume.Properties{
			"property-name":    "property-value",
			"another-property": "yet-another-value",
		}))

	})

})
