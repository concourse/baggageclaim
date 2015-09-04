package volume_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
)

var _ = Describe("Repository Locking", func() {
	var (
		volumeDir  string
		fakeDriver *fakes.FakeDriver
		fakeLocker *fakes.Locker
		logger     lager.Logger

		unlockChan chan interface{}
		lockChan   chan interface{}
		locked     chan interface{}
	)

	zero := uint(0)

	BeforeEach(func() {
		var err error
		volumeDir, err = ioutil.TempDir("", fmt.Sprintf("baggageclaim_volume_dir_%d", GinkgoParallelNode()))
		Ω(err).ShouldNot(HaveOccurred())

		unlockChan = make(chan interface{}, 1)
		lockChan = make(chan interface{}, 1)
		locked = make(chan interface{}, 1)

		fakeLocker = fakes.NewLocker(
			volume.NewLocker(),
			lockChan,
			locked,
			unlockChan,
		)
		fakeDriver = new(fakes.FakeDriver)
		logger = lagertest.NewTestLogger("repo-locking")
	})

	AfterEach(func() {
		err := os.RemoveAll(volumeDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	FIt("cannot set TTL on a volume that's having it's properties set", func() {
		repo := volume.NewRepository(logger, fakeDriver, fakeLocker, volumeDir, volume.TTL(60))

		createdVolume, err := repo.CreateVolume(volume.Strategy{"type": volume.StrategyEmpty}, volume.Properties{}, &zero)
		Ω(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			err = repo.SetProperty(createdVolume.Handle, "some", "property")
			Ω(err).ShouldNot(HaveOccurred())
		}()

		lockChan <- nil
		<-locked

		ready := make(chan interface{}, 1)
		go func() {
			defer GinkgoRecover()
			close(ready)
			err = repo.SetTTL(createdVolume.Handle, 1)
			Ω(err).ShouldNot(HaveOccurred())
		}()

		<-ready

		Ω(fakeLocker.LockCallCount()).Should(Equal(2))
		Ω(fakeLocker.UnlockCallCount()).Should(Equal(1))
		unlockChan <- nil
		<-locked
		Ω(fakeLocker.UnlockCallCount()).Should(Equal(2))
	})

})
