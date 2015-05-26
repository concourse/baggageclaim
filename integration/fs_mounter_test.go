package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("FS Mounter", func() {
	if runtime.GOOS != "linux" {
		fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
		return
	}

	var (
		tempDir   string
		diskImage string
		mountPath string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "fs_mounter_test")
		Ω(err).ShouldNot(HaveOccurred())

		diskImage = filepath.Join(tempDir, "image.img")
		mountPath = filepath.Join(tempDir, "mount")

		command := exec.Command(
			fsMounterPath,
			"-diskImage", diskImage,
			"-mountPath", mountPath,
			"-sizeInMegabytes", "100",
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, "10s").Should(gexec.Exit(0))
	})

	AfterEach(func() {
		command := exec.Command(
			fsMounterPath,
			"-diskImage", diskImage,
			"-mountPath", mountPath,
			"-remove",
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, "10s").Should(gexec.Exit(0))

		err = os.RemoveAll(tempDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	It("works", func() {
		command := exec.Command(
			"btrfs",
			"subvolume",
			"show",
			mountPath,
		)

		session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session).Should(gexec.Exit(0))
	})
})
