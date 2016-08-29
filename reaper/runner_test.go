package reaper_test

import (
	"errors"
	"os"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim/reaper"
	"github.com/tedsuo/ifrit"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner", func() {
	var (
		fakeClock *fakeclock.FakeClock
		interval  time.Duration

		reapResults chan<- error

		process ifrit.Process
	)

	BeforeEach(func() {
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		interval = 10 * time.Second

		results := make(chan error)
		reapResults = results

		process = ifrit.Invoke(reaper.NewRunner(
			lagertest.NewTestLogger("test"),
			fakeClock,
			interval,
			func(lager.Logger) error {
				return <-results
			},
		))
	})

	AfterEach(func() {
		close(reapResults)
		process.Signal(os.Interrupt)
		Expect(<-process.Wait()).ToNot(HaveOccurred())
	})

	Context("when the interval elapses", func() {
		BeforeEach(func() {
			fakeClock.Increment(interval)
		})

		It("reaps", func() {
			reapResults <- nil // block on caller receiving the value
		})

		Context("when the interval elapses again", func() {
			BeforeEach(func() {
				fakeClock.Increment(interval)
			})

			It("carries on with the reaping", func() {
				reapResults <- nil
			})
		})

		Context("when reaping fails", func() {
			BeforeEach(func() {
				reapResults <- errors.New("nope")
			})

			It("does not exit", func() {
				Consistently(process.Wait()).ShouldNot(Receive())
			})

			Context("when the interval elapses again", func() {
				BeforeEach(func() {
					fakeClock.Increment(interval)
				})

				It("carries on with the reaping", func() {
					reapResults <- nil
				})
			})
		})
	})

	Context("when the interval has not elapsed", func() {
		BeforeEach(func() {
			fakeClock.Increment(interval - 1)
		})

		It("does not reap", func() {
			Consistently(reapResults).ShouldNot(BeSent(errors.New("should not run")))
		})
	})
})
