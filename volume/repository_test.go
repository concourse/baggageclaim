package volume_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Repository", func() {
	Describe("CreateVolume", func() {
		Describe("BtrFS", func() {
			if runtime.GOOS != "linux" {
				fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
				return
			}

			var tempDir string
			var fsDriver *driver.BtrFSDriver
			var filesystem *fs.BtrfsFilesystem

			BeforeEach(func() {
				var err error
				tempDir, err = ioutil.TempDir("", "baggageclaim_repo_test")
				Ω(err).ShouldNot(HaveOccurred())

				logger := lagertest.NewTestLogger("driver")
				fsDriver = driver.NewBtrFSDriver(logger)
			})

			AfterEach(func() {
				err := filesystem.Delete()
				Ω(err).ShouldNot(HaveOccurred())

				err = os.RemoveAll(tempDir)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("cows", func() {
				logger := lagertest.NewTestLogger("repo")

				imagePath := filepath.Join(tempDir, "image.img")
				rootPath := filepath.Join(tempDir, "mountpoint")
				filesystem = fs.New(logger, imagePath, rootPath)
				err := filesystem.Create(100 * 1024 * 1024)

				Ω(err).ShouldNot(HaveOccurred())

				repo := volume.NewRepository(logger, rootPath, fsDriver)

				parentVolume, err := repo.CreateVolume(volume.Strategy{
					"type": volume.StrategyEmpty,
				}, volume.Properties{})
				Ω(err).ShouldNot(HaveOccurred())

				childVolume, err := repo.CreateVolume(volume.Strategy{
					"type":   volume.StrategyCopyOnWrite,
					"volume": parentVolume.GUID,
				}, volume.Properties{})
				Ω(err).ShouldNot(HaveOccurred())

				childFilePath := filepath.Join(childVolume.Path, "this-should-only-be-in-the-child")
				err = ioutil.WriteFile(childFilePath, []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(childFilePath).Should(BeARegularFile())

				parentFilePath := filepath.Join(parentVolume.Path, "this-should-only-be-in-the-child")
				Ω(parentFilePath).ShouldNot(BeAnExistingFile())
			})
		})
	})
})
