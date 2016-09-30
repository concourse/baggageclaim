package integration_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim"
)

var _ = Describe("Import Strategy", func() {
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

	Describe("API", func() {
		Describe("POST /volumes", func() {
			var (
				volume baggageclaim.Volume
				tmpdir string
			)

			BeforeEach(func() {
				var err error
				tmpdir, err = ioutil.TempDir("", "host_path")
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tmpdir, "file-with-perms"), []byte("file-with-perms-contents"), 0600)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tmpdir, "some-file"), []byte("some-file-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(tmpdir, "some-dir"), 0755)
				Expect(err).NotTo(HaveOccurred())

				err = ioutil.WriteFile(filepath.Join(tmpdir, "some-dir", "file-in-dir"), []byte("file-in-dir-contents"), 0644)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(tmpdir, "empty-dir"), 0755)
				Expect(err).NotTo(HaveOccurred())

				err = os.MkdirAll(filepath.Join(tmpdir, "dir-with-perms"), 0700)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				volume, err = client.CreateVolume(logger, "some-handle", baggageclaim.VolumeSpec{
					Strategy: baggageclaim.ImportStrategy{
						Path: tmpdir,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("created directory", func() {
				var (
					createdDir string
				)

				JustBeforeEach(func() {
					createdDir = volume.Path()
				})

				It("is in the volume dir", func() {
					Expect(createdDir).To(HavePrefix(runner.VolumeDir()))
				})

				It("creates the directory with the correct contents", func() {
					Expect(createdDir).To(BeADirectory())

					Expect(filepath.Join(createdDir, "some-file")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "some-file"))).To(Equal([]byte("some-file-contents")))

					Expect(filepath.Join(createdDir, "file-with-perms")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "file-with-perms"))).To(Equal([]byte("file-with-perms-contents")))
					fi, err := os.Lstat(filepath.Join(createdDir, "file-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err := os.Lstat(filepath.Join(tmpdir, "file-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))

					Expect(filepath.Join(createdDir, "some-dir")).To(BeADirectory())

					Expect(filepath.Join(createdDir, "some-dir", "file-in-dir")).To(BeARegularFile())
					Expect(ioutil.ReadFile(filepath.Join(createdDir, "some-dir", "file-in-dir"))).To(Equal([]byte("file-in-dir-contents")))
					fi, err = os.Lstat(filepath.Join(createdDir, "some-dir", "file-in-dir"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err = os.Lstat(filepath.Join(tmpdir, "some-dir", "file-in-dir"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))
					Expect(filepath.Join(createdDir, "empty-dir")).To(BeADirectory())

					Expect(filepath.Join(createdDir, "dir-with-perms")).To(BeADirectory())
					fi, err = os.Lstat(filepath.Join(createdDir, "dir-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					expectedFI, err = os.Lstat(filepath.Join(tmpdir, "dir-with-perms"))
					Expect(err).NotTo(HaveOccurred())
					Expect(fi.Mode()).To(Equal(expectedFI.Mode()))
				})
			})
		})
	})
})
