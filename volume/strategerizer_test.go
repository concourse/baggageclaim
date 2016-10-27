package volume_test

import (
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	"github.com/concourse/baggageclaim/uidgid/fake_namespacer"
	"github.com/concourse/baggageclaim/volume"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strategerizer", func() {
	var (
		fakeNamespacer *fake_namespacer.FakeNamespacer

		strategerizer volume.Strategerizer
	)

	BeforeEach(func() {
		fakeNamespacer = new(fake_namespacer.FakeNamespacer)

		strategerizer = volume.NewStrategerizer(fakeNamespacer)
	})

	Describe("StrategyFor", func() {
		var (
			request baggageclaim.VolumeRequest

			strategy       volume.Strategy
			strategyForErr error
		)

		BeforeEach(func() {
			request = baggageclaim.VolumeRequest{}
		})

		JustBeforeEach(func() {
			strategy, strategyForErr = strategerizer.StrategyFor(request)
		})

		Context("when privileged", func() {
			BeforeEach(func() {
				request.Privileged = true
			})

			Context("with an empty strategy", func() {
				BeforeEach(func() {
					request.Strategy = baggageclaim.EmptyStrategy{}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs an empty strategy", func() {
					Expect(strategy).To(Equal(volume.EmptyStrategy{}))
				})
			})

			Context("with a COW strategy", func() {
				BeforeEach(func() {
					volume := new(baggageclaimfakes.FakeVolume)
					volume.HandleReturns("parent-handle")
					request.Strategy = baggageclaim.COWStrategy{volume}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs a COW strategy", func() {
					Expect(strategy).To(Equal(volume.COWStrategy{"parent-handle"}))
				})
			})
		})

		Context("when not privileged", func() {
			BeforeEach(func() {
				request.Privileged = false
			})

			Context("with a empty strategy", func() {
				BeforeEach(func() {
					request.Strategy = baggageclaim.EmptyStrategy{}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs a namespaced empty strategy", func() {
					Expect(strategy).To(Equal(volume.NamespacedStrategy{
						PreStrategy: volume.EmptyStrategy{},
						Namespacer:  fakeNamespacer,
					}))
				})
			})

			Context("with a COW strategy", func() {
				BeforeEach(func() {
					volume := new(baggageclaimfakes.FakeVolume)
					volume.HandleReturns("parent-handle")
					request.Strategy = baggageclaim.COWStrategy{volume}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs a namespaced COW strategy", func() {
					Expect(strategy).To(Equal(volume.NamespacedStrategy{
						PreStrategy: volume.COWStrategy{"parent-handle"},
						Namespacer:  fakeNamespacer,
					}))
				})
			})
		})
	})
})
