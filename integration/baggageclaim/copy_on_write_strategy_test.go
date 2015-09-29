package integration_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/uidjunk"
)

var _ = Describe("Copy On Write Strategy", func() {
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
		writeData := func(volumePath string) string {
			filename := randSeq(10)
			newFilePath := filepath.Join(volumePath, filename)

			err := ioutil.WriteFile(newFilePath, []byte(filename), 0755)
			Ω(err).NotTo(HaveOccurred())

			return filename
		}

		dataExistsInVolume := func(filename, volumePath string) bool {
			_, err := os.Stat(filepath.Join(volumePath, filename))
			return err == nil
		}

		Describe("POST /volumes with strategy: cow", func() {
			It("creates a copy of the volume", func() {
				parentVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
					TTLInSeconds: 3600,
				})
				Ω(err).ShouldNot(HaveOccurred())

				dataInParent := writeData(parentVolume.Path())
				Ω(dataExistsInVolume(dataInParent, parentVolume.Path())).To(BeTrue())

				childVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
					Strategy: baggageclaim.COWStrategy{
						Parent: parentVolume,
					},
					TTLInSeconds: 3600,
				})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(dataExistsInVolume(dataInParent, childVolume.Path())).To(BeTrue())

				newDataInParent := writeData(parentVolume.Path())
				Ω(dataExistsInVolume(newDataInParent, parentVolume.Path())).To(BeTrue())
				Ω(dataExistsInVolume(newDataInParent, childVolume.Path())).To(BeFalse())

				dataInChild := writeData(childVolume.Path())
				Ω(dataExistsInVolume(dataInChild, childVolume.Path())).To(BeTrue())
				Ω(dataExistsInVolume(dataInChild, parentVolume.Path())).To(BeFalse())
			})

			Context("when not privileged", func() {
				It("maps uid 0 to (MAX_UID)", func() {
					if runtime.GOOS != "linux" {
						Skip("only runs somewhere we can run privileged")
						return
					}

					parentVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
						TTLInSeconds: 3600,
					})
					Ω(err).ShouldNot(HaveOccurred())

					dataInParent := writeData(parentVolume.Path())
					Ω(dataExistsInVolume(dataInParent, parentVolume.Path())).To(BeTrue())

					childVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
						Strategy: baggageclaim.COWStrategy{
							Parent: parentVolume,
						},
						Privileged:   false,
						TTLInSeconds: 3600,
					})
					Ω(err).ShouldNot(HaveOccurred())

					stat, err := os.Stat(filepath.Join(childVolume.Path(), dataInParent))
					Expect(err).ToNot(HaveOccurred())

					maxUID := uidjunk.MustGetMaxValidUID()
					maxGID := uidjunk.MustGetMaxValidGID()

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(maxUID)))
					Expect(sysStat.Gid).To(Equal(uint32(maxGID)))
				})
			})

			Context("when privileged", func() {
				It("maps uid 0 to uid 0 (no namespacing)", func() {
					if runtime.GOOS != "linux" {
						Skip("only runs somewhere we can run privileged")
						return
					}

					parentVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
						TTLInSeconds: 3600,
					})
					Ω(err).ShouldNot(HaveOccurred())

					dataInParent := writeData(parentVolume.Path())
					Ω(dataExistsInVolume(dataInParent, parentVolume.Path())).To(BeTrue())

					childVolume, err := client.CreateVolume(logger, baggageclaim.VolumeSpec{
						Strategy: baggageclaim.COWStrategy{
							Parent: parentVolume,
						},
						Privileged:   true,
						TTLInSeconds: 3600,
					})
					Ω(err).ShouldNot(HaveOccurred())

					stat, err := os.Stat(filepath.Join(childVolume.Path(), dataInParent))
					Expect(err).ToNot(HaveOccurred())

					sysStat := stat.Sys().(*syscall.Stat_t)
					Expect(sysStat.Uid).To(Equal(uint32(0)))
					Expect(sysStat.Gid).To(Equal(uint32(0)))
				})
			})
		})
	})
})

func randSeq(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
