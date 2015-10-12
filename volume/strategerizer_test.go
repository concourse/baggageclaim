package volume_test

import (
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
	"github.com/concourse/baggageclaim/uidjunk/fake_namespacer"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Strategerizer", func() {
	var (
		fakeNamespacer  *fake_namespacer.FakeNamespacer
		fakeLockManager *fakes.FakeLockManager

		strategerizer volume.Strategerizer
	)

	BeforeEach(func() {
		fakeNamespacer = new(fake_namespacer.FakeNamespacer)
		fakeLockManager = new(fakes.FakeLockManager)

		strategerizer = volume.NewStrategerizer(fakeNamespacer, fakeLockManager)
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
					volume := new(bfakes.FakeVolume)
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

			Context("with a Docker Image strategy", func() {
				BeforeEach(func() {
					request.Strategy = baggageclaim.DockerImageStrategy{
						Repository:  "some/repository",
						Tag:         "some-tag",
						RegistryURL: "some-registry-url",
						Username:    "some-username",
						Password:    "some-password",
					}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs a Docker Image strategy", func() {
					Expect(strategy).To(Equal(volume.DockerImageStrategy{
						LockManager: fakeLockManager,

						Repository:  "some/repository",
						Tag:         "some-tag",
						RegistryURL: "some-registry-url",
						Username:    "some-username",
						Password:    "some-password",
					}))
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
					volume := new(bfakes.FakeVolume)
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

			Context("with a Docker Image strategy", func() {
				BeforeEach(func() {
					request.Strategy = baggageclaim.DockerImageStrategy{
						Repository:  "some/repository",
						Tag:         "some-tag",
						RegistryURL: "some-registry-url",
						Username:    "some-username",
						Password:    "some-password",
					}.Encode()
				})

				It("succeeds", func() {
					Expect(strategyForErr).ToNot(HaveOccurred())
				})

				It("constructs a namespaced Docker Image strategy", func() {
					Expect(strategy).To(Equal(volume.NamespacedStrategy{
						PreStrategy: volume.DockerImageStrategy{
							LockManager: fakeLockManager,

							Repository:  "some/repository",
							Tag:         "some-tag",
							RegistryURL: "some-registry-url",
							Username:    "some-username",
							Password:    "some-password",
						},
						Namespacer: fakeNamespacer,
					}))
				})
			})
		})
	})
})
