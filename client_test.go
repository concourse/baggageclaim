package baggageclaim_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/baggageclaim/client"
)

var _ = Describe("getting the heartbeat interval from a TTL", func() {
	It("has an upper bound of 1 minute", func() {
		ttlInSeconds := uint(500)
		interval := client.IntervalForTTL(ttlInSeconds)

		Expect(interval).To(Equal(time.Minute))
	})

	Context("when the TTL is small", func() {
		It("Returns an interval that is half of the TTL", func() {
			ttlInSeconds := uint(5)
			interval := client.IntervalForTTL(ttlInSeconds)

			Expect(interval).To(Equal(2500 * time.Millisecond))
		})
	})
})
