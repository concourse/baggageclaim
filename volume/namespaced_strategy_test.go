package volume_test

import (
	"errors"

	"github.com/concourse/baggageclaim/uidjunk/fake_namespacer"
	. "github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NamespacedStrategy", func() {
	var (
		fakeStrategy   *fakes.FakeStrategy
		fakeNamespacer *fake_namespacer.FakeNamespacer

		strategy Strategy
	)

	BeforeEach(func() {
		fakeStrategy = new(fakes.FakeStrategy)
		fakeNamespacer = new(fake_namespacer.FakeNamespacer)

		strategy = NamespacedStrategy{
			PreStrategy: fakeStrategy,
			Namespacer:  fakeNamespacer,
		}
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
			materializedVolume, materializeErr = strategy.Materialize("some-volume", fakeFilesystem)
		})

		Context("when materializing in the sub-strategy succeeds", func() {
			var fakeVolume *fakes.FakeFilesystemInitVolume

			BeforeEach(func() {
				fakeVolume = new(fakes.FakeFilesystemInitVolume)
				fakeVolume.DataPathReturns("some-data-path")
				fakeStrategy.MaterializeReturns(fakeVolume, nil)
			})

			Context("when namespacing the data dir succeeds", func() {
				BeforeEach(func() {
					fakeNamespacer.NamespaceReturns(nil)
				})

				It("succeeds", func() {
					Expect(materializeErr).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					Expect(materializedVolume).To(Equal(fakeVolume))
				})

				It("materialized it with the correct handle and filesystem", func() {
					handle, fs := fakeStrategy.MaterializeArgsForCall(0)
					Expect(handle).To(Equal("some-volume"))
					Expect(fs).To(Equal(fakeFilesystem))
				})

				It("namespaced the data path", func() {
					path := fakeNamespacer.NamespaceArgsForCall(0)
					Expect(path).To(Equal("some-data-path"))
				})
			})

			Context("when namespacing fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeNamespacer.NamespaceReturns(disaster)
				})

				It("returns the error", func() {
					Expect(materializeErr).To(Equal(disaster))
				})

				It("destroys the materialized volume", func() {
					Expect(fakeVolume.DestroyCallCount()).To(Equal(1))
				})
			})
		})

		Context("when materializing the volume fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeStrategy.MaterializeReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(materializeErr).To(Equal(disaster))
			})
		})
	})
})
