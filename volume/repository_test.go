package volume_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/baggageclaim/uidjunk/fake_namespacer"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
	"github.com/concourse/baggageclaim/volume/fakes"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Repository", func() {
	var (
		volumeDir      string
		fakeLocker     *fakes.FakeLockManager
		fakeNamespacer *fake_namespacer.FakeNamespacer
	)

	BeforeEach(func() {
		var err error
		volumeDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
		Expect(err).NotTo(HaveOccurred())

		fakeLocker = new(fakes.FakeLockManager)
		fakeNamespacer = new(fake_namespacer.FakeNamespacer)
	})

	AfterEach(func() {
		err := os.RemoveAll(volumeDir)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("naive", func() {
		var (
			fakeDriver *fakes.FakeDriver
			repo       volume.Repository
		)

		BeforeEach(func() {
			var err error
			fakeDriver = new(fakes.FakeDriver)
			logger := lagertest.NewTestLogger("repo")
			repo, err = volume.NewRepository(logger, fakeDriver, fakeLocker, fakeNamespacer, volumeDir)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("creating a volume", func() {
			Context("unprivileged", func() {
				It("namespaces the volume during initialization", func() {
					vol, err := repo.CreateVolume(volume.Strategy{
						"type": volume.StrategyEmpty,
					}, volume.Properties{}, 0, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeNamespacer.NamespaceCallCount()).To(Equal(1))

					initDataDir := filepath.Join(volumeDir, "init", vol.Handle, "volume")
					Expect(fakeNamespacer.NamespaceArgsForCall(0)).To(Equal(initDataDir))
				})
			})

			Context("privileged", func() {
				It("does not namespace the volume", func() {
					_, err := repo.CreateVolume(volume.Strategy{
						"type": volume.StrategyEmpty,
					}, volume.Properties{}, 0, true)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeNamespacer.NamespaceCallCount()).To(Equal(0))
				})
			})
		})

		Context("with a volume", func() {
			var (
				someVolume volume.Volume
			)

			BeforeEach(func() {
				var err error
				someVolume, err = repo.CreateVolume(volume.Strategy{
					"type": volume.StrategyEmpty,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("destroying the volume", func() {
				It("calls DestroyVolume on the driver", func() {
					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).To(BeADirectory())

					err := repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDriver.DestroyVolumeCallCount()).To(Equal(1))
					volumePath := fakeDriver.DestroyVolumeArgsForCall(0)

					tombstone := filepath.Join(volumeDir, "dead", someVolume.Handle, "volume")
					Expect(volumePath).To(Equal(tombstone))
				})

				It("deletes it from the disk", func() {
					volumes, err := repo.ListVolumes(volume.Properties{})
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(1))

					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).To(BeADirectory())

					err = repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					volumes, err = repo.ListVolumes(volume.Properties{})
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(0))

					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).NotTo(BeADirectory())
				})

				It("removes it from listVolumes", func() {
					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).To(BeADirectory())

					err := repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).NotTo(BeADirectory())
					Expect(filepath.Join(volumeDir, "dead", someVolume.Handle)).NotTo(BeADirectory())

					Expect(repo.ListVolumes(volume.Properties{})).To(BeEmpty())
				})

				It("makes some attempt at locking", func() {
					err := repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})

			Describe("setting properties on the volume", func() {
				It("makes some attempt at locking", func() {
					err := repo.SetProperty(someVolume.Handle, "hello", "goodbye")
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})

			Describe("setting the TTL on the volume", func() {
				It("makes some attempt at locking", func() {
					err := repo.SetTTL(someVolume.Handle, 42)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})
		})
	})

	Describe("BtrFS", func() {
		if runtime.GOOS != "linux" {
			fmt.Println("\x1b[33m*** skipping btrfs tests because non-linux ***\x1b[0m")
			return
		}

		var (
			tempDir    string
			volumeDir  string
			fsDriver   *driver.BtrFSDriver
			filesystem *fs.BtrfsFilesystem
			repo       volume.Repository
		)

		BeforeEach(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "baggageclaim_repo_test")
			Expect(err).NotTo(HaveOccurred())

			dLogger := lagertest.NewTestLogger("driver")
			fsDriver = driver.NewBtrFSDriver(dLogger)

			imagePath := filepath.Join(tempDir, "image.img")
			volumeDir = filepath.Join(tempDir, "mountpoint")
			filesystem = fs.New(dLogger, imagePath, volumeDir)
			err = filesystem.Create(100 * 1024 * 1024)
			Expect(err).NotTo(HaveOccurred())

			logger := lagertest.NewTestLogger("repo")
			repo, err = volume.NewRepository(logger, fsDriver, fakeLocker, fakeNamespacer, volumeDir)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := filesystem.Delete()
			Expect(err).NotTo(HaveOccurred())

			err = os.RemoveAll(tempDir)
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("creating a new volume", func() {
			It("cows", func() {
				parentVolume, err := repo.CreateVolume(volume.Strategy{
					"type": volume.StrategyEmpty,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())

				childVolume, err := repo.CreateVolume(volume.Strategy{
					"type":   volume.StrategyCopyOnWrite,
					"volume": parentVolume.Handle,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())

				childsParentFile := filepath.Join(volumeDir, "live", childVolume.Handle, "parent")
				Expect(childsParentFile).To(BeADirectory())

				parentPath, err := filepath.EvalSymlinks(childsParentFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(parentPath).To(Equal(filepath.Join(volumeDir, "live", parentVolume.Handle)))

				childFilePath := filepath.Join(childVolume.Path, "this-should-only-be-in-the-child")
				err = ioutil.WriteFile(childFilePath, []byte("contents"), 0755)
				Expect(err).NotTo(HaveOccurred())
				Expect(childFilePath).To(BeARegularFile())

				parentFilePath := filepath.Join(parentVolume.Path, "this-should-only-be-in-the-child")
				Expect(parentFilePath).NotTo(BeADirectory())
			})

			It("makes some attempt at locking the parent", func() {
				parentVolume, err := repo.CreateVolume(volume.Strategy{
					"type": volume.StrategyEmpty,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())

				_, err = repo.CreateVolume(volume.Strategy{
					"type":   volume.StrategyCopyOnWrite,
					"volume": parentVolume.Handle,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeLocker.LockCallCount()).To(Equal(1))
				Expect(fakeLocker.LockArgsForCall(0)).To(Equal(parentVolume.Handle))
				Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
				Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(parentVolume.Handle))
			})

			Context("unprivileged", func() {
				It("namespaces the volume during initialization", func() {
					parentVolume, err := repo.CreateVolume(volume.Strategy{
						"type": volume.StrategyEmpty,
					}, volume.Properties{}, 0, true)
					Expect(err).NotTo(HaveOccurred())

					vol, err := repo.CreateVolume(volume.Strategy{
						"type":   volume.StrategyCopyOnWrite,
						"volume": parentVolume.Handle,
					}, volume.Properties{}, 0, false)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeNamespacer.NamespaceCallCount()).To(Equal(1))

					initDataDir := filepath.Join(volumeDir, "init", vol.Handle, "volume")
					Expect(fakeNamespacer.NamespaceArgsForCall(0)).To(Equal(initDataDir))
				})
			})

			Context("privileged", func() {
				It("does not namespace the volume", func() {
					parentVolume, err := repo.CreateVolume(volume.Strategy{
						"type": volume.StrategyEmpty,
					}, volume.Properties{}, 0, true)
					Expect(err).NotTo(HaveOccurred())

					_, err = repo.CreateVolume(volume.Strategy{
						"type":   volume.StrategyCopyOnWrite,
						"volume": parentVolume.Handle,
					}, volume.Properties{}, 0, true)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeNamespacer.NamespaceCallCount()).To(Equal(0))
				})
			})
		})

		Context("with a volume", func() {
			var (
				someVolume volume.Volume
			)

			BeforeEach(func() {
				var err error
				someVolume, err = repo.CreateVolume(volume.Strategy{
					"type": volume.StrategyEmpty,
				}, volume.Properties{}, 0, false)
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("destroying the volume", func() {
				It("deletes it", func() {
					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).To(BeADirectory())

					err := repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					volumes, err := repo.ListVolumes(volume.Properties{})
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(0))

					Expect(filepath.Join(volumeDir, "live", someVolume.Handle)).NotTo(BeADirectory())
				})

				It("makes some attempt at locking", func() {
					someVolume, err := repo.CreateVolume(volume.Strategy{
						"type": volume.StrategyEmpty,
					}, volume.Properties{}, 0, false)
					Expect(err).NotTo(HaveOccurred())

					err = repo.DestroyVolume(someVolume.Handle)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})

			Describe("setting properties on the volume", func() {
				It("makes some attempt at locking", func() {
					err := repo.SetProperty(someVolume.Handle, "hello", "goodbye")
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})

			Describe("setting the TTL on the volume", func() {
				It("makes some attempt at locking", func() {
					err := repo.SetTTL(someVolume.Handle, 42)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeLocker.LockCallCount()).To(Equal(1))
					Expect(fakeLocker.LockArgsForCall(0)).To(Equal(someVolume.Handle))
					Expect(fakeLocker.UnlockCallCount()).To(Equal(1))
					Expect(fakeLocker.UnlockArgsForCall(0)).To(Equal(someVolume.Handle))
				})
			})
		})
	})
})
