package volume

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume Promise", func() {
	var (
		promise Promise
	)

	BeforeEach(func() {
		promise = NewPromise()
	})

	Context("newly created", func() {
		It("is pending", func() {
			Expect(promise.IsPending()).To(BeTrue())
		})

		It("can not return a value yet", func() {
			_, _, err := promise.GetValue()

			Expect(err).To(Equal(ErrPromiseStillPending))
		})
	})

	Context("when fulfilled", func() {
		It("is not pending", func() {
			promise.Fulfill(Volume{})

			Expect(promise.IsPending()).To(BeFalse())
		})

		It("returns a non-empty volume in value", func() {
			volume := Volume{
				Handle:     "test-handle",
				Path:       "test-path",
				Properties: make(Properties),
			}

			promise.Fulfill(volume)

			val, _, _ := promise.GetValue()

			Expect(val).To(Equal(volume))
		})

		It("returns a nil error in value", func() {
			volume := Volume{
				Handle:     "test-handle",
				Path:       "test-path",
				Properties: make(Properties),
			}

			promise.Fulfill(volume)

			_, val, _ := promise.GetValue()

			Expect(val).To(BeNil())
		})

		It("can return a value", func() {
			volume := Volume{
				Handle:     "test-handle",
				Path:       "test-path",
				Properties: make(Properties),
			}

			promise.Fulfill(volume)

			_, _, err := promise.GetValue()

			Expect(err).To(BeNil())
		})
	})

	Context("when rejected", func() {
		var (
			testErr = errors.New("test-error")
		)

		It("is not pending", func() {
			promise.Reject(testErr)

			Expect(promise.IsPending()).To(BeFalse())
		})

		It("returns an empty volume in value", func() {
			promise.Reject(testErr)

			val, _, _ := promise.GetValue()

			Expect(val).To(Equal(Volume{}))
		})

		It("returns a non-nil error in value", func() {
			promise.Reject(testErr)

			_, val, _ := promise.GetValue()

			Expect(val).To(Equal(testErr))
		})

		It("can return a value", func() {
			promise.Reject(testErr)

			_, _, err := promise.GetValue()

			Expect(err).To(BeNil())
		})

		Context("when rejecting again", func() {
			Context("when canceled", func() {
				It("returns ErrPromiseCanceled", func() {
					promise.Reject(ErrPromiseCanceled)

					err := promise.Reject(testErr)

					Expect(err).To(Equal(ErrPromiseCanceled))
				})
			})

			Context("when not canceled", func() {
				It("returns ErrPromiseNotPending", func() {
					promise.Reject(testErr)

					err := promise.Reject(testErr)

					Expect(err).To(Equal(ErrPromiseNotPending))
				})
			})
		})
	})
})
