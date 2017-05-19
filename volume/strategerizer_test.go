package volume_test

import (
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	"github.com/concourse/baggageclaim/volume"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strategerizer", func() {
	var (
		strategerizer volume.Strategerizer
	)

	BeforeEach(func() {
		strategerizer = volume.NewStrategerizer()
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
})
