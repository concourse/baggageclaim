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

func mountAtPath(path string) string {
	diskImage := filepath.Join(path, "image.img")
	mountPath := filepath.Join(path, "mount")

	command := exec.Command(
		fsMounterPath,
		"-diskImage", diskImage,
		"-mountPath", mountPath,
		"-sizeInMegabytes", "100",
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(session, "10s").Should(gexec.Exit(0))

	return mountPath
}

func unmountAtPath(path string) {
	diskImage := filepath.Join(path, "image.img")
	mountPath := filepath.Join(path, "mount")

	command := exec.Command(
		fsMounterPath,
		"-diskImage", diskImage,
		"-mountPath", mountPath,
		"-remove",
	)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	Eventually(session, "10s").Should(gexec.Exit(0))

}

var _ = Describe("FS Mounter", func() {
	if runtime.GOOS != "linux" {
		fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
		return
	}

	var (
		tempDir   string
		mountPath string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = ioutil.TempDir("", "fs_mounter_test")
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		err := os.RemoveAll(tempDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	Context("when starting for the first time", func() {
		BeforeEach(func() {
			mountPath = mountAtPath(tempDir)
		})

		AfterEach(func() {
			unmountAtPath(tempDir)
		})

		It("mounts a btrfs volume", func() {
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

	Context("on subsequent runs", func() {
		BeforeEach(func() {
			mountPath = mountAtPath(tempDir)
		})

		AfterEach(func() {
			unmountAtPath(tempDir)
		})

		It("is idepotent", func() {
			path := filepath.Join(mountPath, "filez")
			err := ioutil.WriteFile(path, []byte("contents"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			mountPath = mountAtPath(tempDir)

			contents, err := ioutil.ReadFile(path)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(string(contents)).Should(Equal("contents"))
		})
	})
})
