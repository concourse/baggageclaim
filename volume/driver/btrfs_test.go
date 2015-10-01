package driver_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/baggageclaim/volume/driver"
)

var _ = Describe("BtrFS", func() {
	if runtime.GOOS != "linux" {
		fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
		return
	}

	var (
		tempDir    string
		volumeDir  string
		fsDriver   *driver.BtrFSDriver
		filesystem *fs.BtrfsFilesystem
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "baggageclaim_driver_test")
		Expect(err).NotTo(HaveOccurred())

		logger := lagertest.NewTestLogger("fs")

		imagePath := filepath.Join(tempDir, "image.img")
		volumeDir = filepath.Join(tempDir, "mountpoint")

		filesystem = fs.New(logger, imagePath, volumeDir)
		err = filesystem.Create(100 * 1024 * 1024)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := filesystem.Delete()
		Expect(err).NotTo(HaveOccurred())

		err = os.RemoveAll(tempDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Lifecycle", func() {
		It("can create and delete a subvolume", func() {
			logger := lagertest.NewTestLogger("driver")
			fsDriver = driver.NewBtrFSDriver(logger)

			subvolumePath := filepath.Join(volumeDir, "subvolume")

			err := fsDriver.CreateVolume(subvolumePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(subvolumePath).To(BeADirectory())

			checkSubvolume := exec.Command("btrfs", "subvolume", "show", subvolumePath)
			session, err := gexec.Start(checkSubvolume, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gbytes.Say(subvolumePath))
			Eventually(session).Should(gexec.Exit(0))

			err = fsDriver.DestroyVolume(subvolumePath)
			Expect(err).NotTo(HaveOccurred())

			Expect(subvolumePath).NotTo(BeADirectory())
		})
	})
})
