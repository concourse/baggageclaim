package client

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Sleeper

type Sleeper interface {
	Sleep(time.Duration)
}

//go:generate counterfeiter . RetryPolicy

type RetryPolicy interface {
	DelayFor(uint) (time.Duration, bool)
}

//go:generate counterfeiter . RoundTripper

type RoundTripper interface {
	RoundTrip(request *http.Request) (*http.Response, error)
}

type RetryRoundTripper struct {
	Logger       lager.Logger
	Sleeper      Sleeper
	RetryPolicy  RetryPolicy
	RoundTripper RoundTripper
}

func (d *RetryRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	retryLogger := d.Logger.Session("retry")
	startTime := time.Now()

	var err error

	var failedAttempts uint
	for {
		var response *http.Response
		response, err = d.RoundTripper.RoundTrip(request)
		if err == nil {
			return response, nil
		}

		if !retryable(err) {
			break
		}

		failedAttempts++

		delay, keepRetrying := d.RetryPolicy.DelayFor(failedAttempts)
		if !keepRetrying {
			retryLogger.Error("giving-up", errors.New("giving up"), lager.Data{
				"total-failed-attempts": failedAttempts,
				"ran-for":               time.Now().Sub(startTime).String(),
			})

			break
		}

		retryLogger.Info("retrying", lager.Data{
			"failed-attempts": failedAttempts,
			"next-attempt-in": delay.String(),
			"ran-for":         time.Now().Sub(startTime).String(),
		})

		d.Sleeper.Sleep(delay)
	}

	return nil, err
}

func retryable(err error) bool {
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}

	s := err.Error()
	for _, retryableError := range retryableErrors {
		if strings.HasSuffix(s, retryableError.Error()) {
			return true
		}
	}

	return false
}

var retryableErrors = []error{
	syscall.ECONNREFUSED,
	syscall.ECONNRESET,
	syscall.ETIMEDOUT,
	errors.New("i/o timeout"),
	errors.New("no such host"),
	errors.New("remote error: handshake failure"),
}
