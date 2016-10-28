package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim"
)

var _ = Describe("reaping corrupted volumes", func() {
	var (
		runner             *BaggageClaimRunner
		client             baggageclaim.Client
		propertiesFileName = "properties.json"
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

	overwriteData := func(path string) error {
		file, err := os.OpenFile(
			path,
			os.O_WRONLY|os.O_CREATE,
			0644,
		)

		if err != nil {
			return err
		}

		defer file.Close()

		err = json.NewEncoder(file).Encode("garbage2")

		if err != nil {
			return err
		}

		return nil
	}

	verifyDestroyed := func(victim baggageclaim.Volume) {
		Eventually(func() bool {
			found := false
			for _, v := range runner.CurrentHandles() {
				if v == victim.Handle() {
					found = true
				}
			}
			return found
		}, 10*time.Second).Should(BeFalse())
	}

	It("destroys corrupt volume and descendants", func() {
		parentVolume, err := client.CreateVolume(logger,
			"some-parent-handle",
			baggageclaim.VolumeSpec{
				Properties: baggageclaim.VolumeProperties{
					"property-name": "property-value",
				},
			})

		Expect(err).NotTo(HaveOccurred())
		childVolume, err := client.CreateVolume(logger,
			"some-child-handle",
			baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: parentVolume,
				},
				Privileged: false,
			})
		Expect(err).NotTo(HaveOccurred())

		propertiesDataPath := filepath.Join(filepath.Dir(parentVolume.Path()), propertiesFileName)

		err = overwriteData(propertiesDataPath)
		Expect(err).NotTo(HaveOccurred())

		verifyDestroyed(parentVolume)
		verifyDestroyed(childVolume)
	})
})
