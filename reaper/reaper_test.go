package reaper_test

import (
	"errors"
	"time"

	. "github.com/concourse/baggageclaim/reaper"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reaper", func() {
	var (
		repository *fakes.FakeRepository
		clock      *fakeclock.FakeClock

		reaper *Reaper
	)

	now := time.Unix(123, 456)

	BeforeEach(func() {
		repository = new(fakes.FakeRepository)
		clock = fakeclock.NewFakeClock(now)

		reaper = NewReaper(clock, repository)
	})

	Describe("Reap", func() {
		var reapErr error

		JustBeforeEach(func() {
			reapErr = reaper.Reap(lagertest.NewTestLogger("test"))
		})

		Context("when listing the volumes works", func() {
			nonExpiringVolume := volume.Volume{
				Handle: "non-expiring",
				TTL:    0,
			}

			expiringVolume10sec := volume.Volume{
				Handle:    "expiring-10sec",
				TTL:       10,
				ExpiresAt: now.Add(10 * time.Second),
			}

			expiringVolume20sec := volume.Volume{
				Handle:    "expiring-20sec",
				TTL:       20,
				ExpiresAt: now.Add(20 * time.Second),
			}

			BeforeEach(func() {
				repository.ListVolumesReturns([]volume.Volume{
					nonExpiringVolume,
					expiringVolume10sec,
					expiringVolume20sec,
				}, nil)
			})

			It("lists volumes with no filter", func() {
				Expect(repository.ListVolumesArgsForCall(0)).To(BeEmpty())
			})

			Context("when no volumes have expired", func() {
				It("does nothin'", func() {
					Expect(repository.DestroyVolumeCallCount()).To(BeZero())
				})
			})

			Context("when a volume has expired", func() {
				BeforeEach(func() {
					clock.Increment(10*time.Second + 1)
				})

				It("destroys it", func() {
					Expect(repository.DestroyVolumeCallCount()).To(Equal(1))

					handle := repository.DestroyVolumeArgsForCall(0)
					Expect(handle).To(Equal(expiringVolume10sec.Handle))
				})

				Context("when determining if a volume has a parent fails", func() {
					BeforeEach(func() {
						repository.VolumeParentReturns(volume.Volume{}, false, errors.New("nope"))
					})

					It("returns an error", func() {
						Expect(reapErr).To(MatchError("failed to determine volume parent: nope"))
					})

					It("does not destroy any volumes", func() {
						Expect(repository.DestroyVolumeCallCount()).To(BeZero())
					})
				})

				Context("when the expired volume has children", func() {
					BeforeEach(func() {
						repository.VolumeParentStub = func(handle string) (volume.Volume, bool, error) {
							switch handle {
							case nonExpiringVolume.Handle:
								return expiringVolume10sec, true, nil
							default:
								return volume.Volume{}, false, nil
							}
						}
					})

					It("is not destroyed", func() {
						Expect(repository.DestroyVolumeCallCount()).To(BeZero())
					})

					Context("regardless of the order in which the volumes are returned", func() {
						BeforeEach(func() {
							repository.ListVolumesReturns([]volume.Volume{
								expiringVolume20sec,
								expiringVolume10sec,
								nonExpiringVolume,
							}, nil)
						})

						It("is not destroyed", func() {
							Expect(repository.DestroyVolumeCallCount()).To(BeZero())
						})
					})
				})
			})

			Context("when multiple volumes have expired", func() {
				BeforeEach(func() {
					clock.Increment(20*time.Second + 1)
				})

				It("destroys the expired volumes", func() {
					Expect(repository.DestroyVolumeCallCount()).To(Equal(2))

					handle1 := repository.DestroyVolumeArgsForCall(0)
					Expect(handle1).To(Equal(expiringVolume10sec.Handle))

					handle2 := repository.DestroyVolumeArgsForCall(1)
					Expect(handle2).To(Equal(expiringVolume20sec.Handle))
				})

				Context("when destroying any volumes fails", func() {
					BeforeEach(func() {
						repository.DestroyVolumeStub = func(handle string) error {
							return errors.New("nope to " + handle)
						}
					})

					It("returns an aggregated error", func() {
						Expect(reapErr).To(HaveOccurred())
						Expect(reapErr.Error()).To(ContainSubstring("failed to destroy expiring-10sec: nope to expiring-10sec"))
						Expect(reapErr.Error()).To(ContainSubstring("failed to destroy expiring-20sec: nope to expiring-20sec"))
					})
				})
			})
		})

		Context("when listing the volumes blows up", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				repository.ListVolumesReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(reapErr).To(MatchError("failed to list volumes: nope"))
			})

			It("does nothin'", func() {
				Expect(repository.DestroyVolumeCallCount()).To(BeZero())
			})
		})
	})
})
