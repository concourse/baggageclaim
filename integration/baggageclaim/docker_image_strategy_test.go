package integration_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim"
)

var _ = Describe("Docker Image Strategy", func() {
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

	Describe("creating a volume from a public image with no auth", func() {
		It("works", func() {
			volume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
				Strategy: baggageclaim.DockerImageStrategy{
					RegistryURL: "https://registry-1.docker.io",
					Repository:  "library/busybox",
					Tag:         "latest",
				},
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(filepath.Join(volume.Path(), "bin", "busybox")).To(BeARegularFile())
		})
	})
})
