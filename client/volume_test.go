package client_test

import (
	"time"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/client"
	"github.com/concourse/baggageclaim/fakes"
	"github.com/concourse/baggageclaim/volume"
	"github.com/pivotal-golang/clock/fakeclock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume", func() {
	var fakeClient *fakes.FakeClient
	var fakeClock *fakeclock.FakeClock
	var vol volume.Volume

	BeforeEach(func() {
		fakeClient = new(fakes.FakeClient)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))

		vol = volume.Volume{
			Handle: "some-handle",
			TTL:    volume.TTL(1),
		}
	})

	It("can heartbeat itself", func() {
		hbVol := client.NewVolume(fakeClient, vol)

		hbVol.Heartbeat(30*time.Second, fakeClock)

		Eventually(fakeClient.SetTTLCallCount).Should(Equal(1))
		handle, ttl := fakeClient.SetTTLArgsForCall(0)
		Ω(handle).Should(Equal("some-handle"))
		Ω(ttl).Should(Equal(uint(1)))

		fakeClock.Increment(30 * time.Second)

		Eventually(fakeClient.SetTTLCallCount).Should(Equal(2))
		handle, ttl = fakeClient.SetTTLArgsForCall(1)
		Ω(handle).Should(Equal("some-handle"))
		Ω(ttl).Should(Equal(uint(1)))

		fakeClock.Increment(30 * time.Second)

		Eventually(fakeClient.SetTTLCallCount).Should(Equal(3))
		handle, ttl = fakeClient.SetTTLArgsForCall(2)
		Ω(handle).Should(Equal("some-handle"))
		Ω(ttl).Should(Equal(uint(1)))

		hbVol.Release()

		fakeClock.Increment(30 * time.Second)

		Consistently(fakeClient.SetTTLCallCount).Should(Equal(3))
	})

	Context("when the volume disappears while heartbeating", func() {
		It("gives up", func() {
			hbVol := client.NewVolume(fakeClient, vol)

			hbVol.Heartbeat(30*time.Second, fakeClock)

			Eventually(fakeClient.SetTTLCallCount).Should(Equal(1))
			handle, ttl := fakeClient.SetTTLArgsForCall(0)
			Ω(handle).Should(Equal("some-handle"))
			Ω(ttl).Should(Equal(uint(1)))

			fakeClock.Increment(30 * time.Second)

			Eventually(fakeClient.SetTTLCallCount).Should(Equal(2))
			handle, ttl = fakeClient.SetTTLArgsForCall(1)
			Ω(handle).Should(Equal("some-handle"))
			Ω(ttl).Should(Equal(uint(1)))

			fakeClient.SetTTLReturns(baggageclaim.ErrVolumeNotFound)

			fakeClock.Increment(30 * time.Second)

			Eventually(fakeClient.SetTTLCallCount).Should(Equal(3))
			handle, ttl = fakeClient.SetTTLArgsForCall(2)
			Ω(handle).Should(Equal("some-handle"))
			Ω(ttl).Should(Equal(uint(1)))

			fakeClock.Increment(30 * time.Second)

			Consistently(fakeClient.SetTTLCallCount).Should(Equal(3))
		})
	})

	Context("when the volume disappears immediately", func() {
		It("gives up", func() {
			hbVol := client.NewVolume(fakeClient, vol)

			fakeClient.SetTTLReturns(baggageclaim.ErrVolumeNotFound)

			hbVol.Heartbeat(30*time.Second, fakeClock)

			Eventually(fakeClient.SetTTLCallCount).Should(Equal(1))
			handle, ttl := fakeClient.SetTTLArgsForCall(0)
			Ω(handle).Should(Equal("some-handle"))
			Ω(ttl).Should(Equal(uint(1)))

			fakeClock.Increment(30 * time.Second)

			Consistently(fakeClient.SetTTLCallCount).Should(Equal(1))
		})
	})
})
