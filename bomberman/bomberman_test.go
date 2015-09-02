package bomberman_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	"github.com/concourse/baggageclaim/bomberman"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/fakes"
)

var _ = Ω

var _ = Describe("Bomberman", func() {
	It("straps a bomb to the given container with the container's grace time as the countdown", func() {
		detonated := make(chan volume.Volume)

		repository := new(fakes.FakeRepository)
		repository.TTLReturns(100 * time.Millisecond)

		bomberman := bomberman.New(repository, func(volume volume.Volume) {
			detonated <- volume
		})

		someVolume := volume.Volume{
			GUID: "some-guid",
		}

		bomberman.Strap(someVolume)

		select {
		case <-detonated:
		case <-time.After(repository.TTL(someVolume) + 50*time.Millisecond):
			Fail("did not detonate!")
		}
	})

	Context("when the container has a grace time of 0", func() {
		It("never detonates", func() {

			detonated := make(chan volume.Volume)

			repository := new(fakes.FakeRepository)
			repository.TTLReturns(0)

			bomberman := bomberman.New(repository, func(volume volume.Volume) {
				detonated <- volume
			})

			someVolume := volume.Volume{
				GUID: "some-guid",
			}

			bomberman.Strap(someVolume)

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(repository.TTL(someVolume) + 50*time.Millisecond):
			}
		})
	})

	Describe("pausing a container's timebomb", func() {
		It("prevents it from detonating", func() {
			detonated := make(chan volume.Volume)

			repository := new(fakes.FakeRepository)
			repository.TTLReturns(100 * time.Millisecond)

			bomberman := bomberman.New(repository, func(volume volume.Volume) {
				detonated <- volume
			})

			someVolume := volume.Volume{
				GUID: "doomed",
			}

			bomberman.Strap(someVolume)
			bomberman.Pause("doomed")

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(repository.TTL(someVolume) + 50*time.Millisecond):
			}
		})

		Context("when the handle is invalid", func() {
			It("doesn't launch any missiles or anything like that", func() {
				bomberman := bomberman.New(new(fakes.FakeRepository), func(volume volume.Volume) {
					panic("dont call me")
				})

				bomberman.Pause("BOOM?!")
			})
		})

		Describe("and then unpausing it", func() {
			It("causes it to detonate after the countdown", func() {

				detonated := make(chan volume.Volume)

				repository := new(fakes.FakeRepository)
				repository.TTLReturns(100 * time.Millisecond)

				bomberman := bomberman.New(repository, func(volume volume.Volume) {
					detonated <- volume
				})

				someVolume := volume.Volume{
					GUID: "doomed",
				}

				bomberman.Strap(someVolume)

				bomberman.Pause("doomed")

				before := time.Now()
				bomberman.Unpause("doomed")

				select {
				case <-detonated:
					Ω(time.Since(before)).Should(BeNumerically(">=", 100*time.Millisecond))
				case <-time.After(repository.TTL(someVolume) + 50*time.Millisecond):
					Fail("did not detonate!")
				}
			})

			Context("when the handle is invalid", func() {
				It("doesn't launch any missiles or anything like that", func() {
					bomberman := bomberman.New(new(fakes.FakeRepository), func(volume volume.Volume) {
						panic("dont call me")
					})

					bomberman.Unpause("BOOM?!")
				})
			})
		})
	})

	Describe("defusing a container's timebomb", func() {
		It("prevents it from detonating", func() {
			detonated := make(chan volume.Volume)

			repository := new(fakes.FakeRepository)
			repository.TTLReturns(100 * time.Millisecond)

			bomberman := bomberman.New(repository, func(volume volume.Volume) {
				detonated <- volume
			})

			someVolume := volume.Volume{
				GUID: "doomed",
			}

			bomberman.Strap(someVolume)
			bomberman.Defuse("doomed")

			select {
			case <-detonated:
				Fail("detonated!")
			case <-time.After(repository.TTL(someVolume) + 50*time.Millisecond):
			}
		})

		Context("when the handle is invalid", func() {
			It("doesn't launch any missiles or anything like that", func() {
				bomberman := bomberman.New(new(fakes.FakeRepository), func(volume volume.Volume) {
					panic("dont call me")
				})

				bomberman.Defuse("BOOM?!")
			})
		})
	})
})
