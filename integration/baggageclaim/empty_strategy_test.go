package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/integration/baggageclaim"
	"github.com/concourse/baggageclaim/volume"
)

var _ = Describe("Empty Strategy", func() {
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

	Describe("API", func() {
		properties := volume.Properties{
			"name": "value",
		}

		Describe("POST /volumes", func() {
			var (
				firstVolume volume.Volume
			)

			JustBeforeEach(func() {
				var err error
				firstVolume, err = client.CreateEmptyVolume(integration.VolumeSpec{})
				Ω(err).ShouldNot(HaveOccurred())
			})

			Describe("created directory", func() {
				var (
					createdDir string
				)

				JustBeforeEach(func() {
					createdDir = firstVolume.Path
				})

				It("is in the volume dir", func() {
					Ω(createdDir).Should(HavePrefix(runner.VolumeDir()))
				})

				It("creates the directory", func() {
					Ω(createdDir).Should(BeADirectory())
				})

				Context("on a second request", func() {
					var (
						secondVolume volume.Volume
					)

					JustBeforeEach(func() {
						var err error
						secondVolume, err = client.CreateEmptyVolume(integration.VolumeSpec{})
						Ω(err).ShouldNot(HaveOccurred())
					})

					It("creates a new directory", func() {
						Ω(createdDir).ShouldNot(Equal(secondVolume.Path))
					})

					It("creates a new handle", func() {
						Ω(firstVolume.Handle).ShouldNot(Equal(secondVolume.Handle))
					})
				})
			})
		})

		Describe("GET /volumes", func() {
			var (
				volumes volume.Volumes
			)

			JustBeforeEach(func() {
				var err error
				volumes, err = client.GetVolumes()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("returns an empty response", func() {
				Ω(volumes).Should(BeEmpty())
			})

			Context("when a volume has been created", func() {
				var createdVolume volume.Volume

				BeforeEach(func() {
					var err error
					createdVolume, err = client.CreateEmptyVolume(integration.VolumeSpec{Properties: properties})
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("returns it", func() {
					Ω(volumes).Should(ConsistOf(createdVolume))
				})
			})
		})
	})
})
