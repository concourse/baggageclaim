package volume_test

import (
	"errors"

	. "github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CowStrategy", func() {
	var (
		strategy Strategy
	)

	BeforeEach(func() {
		strategy = CowStrategy{"parent-volume"}
	})

	Describe("Materialize", func() {
		var (
			fakeFilesystem *fakes.FakeFilesystem

			materializedVolume FilesystemInitVolume
			materializeErr     error
		)

		BeforeEach(func() {
			fakeFilesystem = new(fakes.FakeFilesystem)
		})

		JustBeforeEach(func() {
			materializedVolume, materializeErr = strategy.Materialize(
				lagertest.NewTestLogger("test"),
				"some-volume",
				fakeFilesystem,
			)
		})

		Context("when the parent volume can be found", func() {
			var parentVolume *fakes.FakeFilesystemLiveVolume

			BeforeEach(func() {
				parentVolume = new(fakes.FakeFilesystemLiveVolume)
				fakeFilesystem.LookupVolumeReturns(parentVolume, true, nil)
			})

			Context("when creating the sub volume succeeds", func() {
				var fakeVolume *fakes.FakeFilesystemInitVolume

				BeforeEach(func() {
					parentVolume.NewSubvolumeReturns(fakeVolume, nil)
				})

				It("succeeds", func() {
					Expect(materializeErr).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					Expect(materializedVolume).To(Equal(fakeVolume))
				})

				It("created it with the correct handle", func() {
					handle := parentVolume.NewSubvolumeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
				})

				It("looked up the parent with the correct handle", func() {
					handle := fakeFilesystem.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("parent-volume"))
				})
			})

			Context("when creating the sub volume fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					parentVolume.NewSubvolumeReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(materializeErr).To(Equal(disaster))
				})
			})
		})

		Context("when no parent volume is given", func() {
			BeforeEach(func() {
				strategy = CowStrategy{""}
			})

			It("returns ErrNoParentVolumeProvided", func() {
				Expect(materializeErr).To(Equal(ErrNoParentVolumeProvided))
			})

			It("does not look it up", func() {
				Expect(fakeFilesystem.LookupVolumeCallCount()).To(Equal(0))
			})
		})

		Context("when the parent handle does not exist", func() {
			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, nil)
			})

			It("returns ErrParentVolumeNotFound", func() {
				Expect(materializeErr).To(Equal(ErrParentVolumeNotFound))
			})
		})

		Context("when looking up the parent volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeFilesystem.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(materializeErr).To(Equal(disaster))
			})
		})
	})
})
