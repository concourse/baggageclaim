package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/concourse/baggageclaim"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Size", func() {
	if runtime.GOOS == "linux" {
		fmt.Println("\x1b[33m*** skipping volume size tests because btrfs ***\x1b[0m")
		return
	}
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

	Describe("Size", func() {
		var cowVolume baggageclaim.Volume
		var parentVolume baggageclaim.Volume

		BeforeEach(func() {
			parentSpec := baggageclaim.VolumeSpec{
				Strategy: baggageclaim.EmptyStrategy{},
				TTL:      10 * time.Second,
			}

			var err error
			parentVolume, err = client.CreateVolume(logger, "parent-handle", parentSpec)
			Expect(err).NotTo(HaveOccurred())

			ioutil.WriteFile(filepath.Join(parentVolume.Path(), "some-parent-file"), []byte("some-bytes"), os.ModePerm)
			ioutil.WriteFile(filepath.Join(parentVolume.Path(), "some-other-parent-file"), []byte("some-bytes"), os.ModePerm)

			cowSpec := baggageclaim.VolumeSpec{
				Strategy: baggageclaim.COWStrategy{
					Parent: parentVolume,
				},
				TTL: 10 * time.Second,
			}

			cowVolume, err = client.CreateVolume(logger, "cow-handle", cowSpec)
			Expect(err).NotTo(HaveOccurred())

			ioutil.WriteFile(filepath.Join(cowVolume.Path(), "some-child-file"), []byte("some-bytes"), os.ModePerm)
		})

		It("returns the size of the volume", func() {
			size, err := parentVolume.SizeInBytes()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(16)))

			size, err = cowVolume.SizeInBytes()
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(24))) // naive driver
		})
	})
})
