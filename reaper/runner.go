package reaper

import (
	"os"
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/tedsuo/ifrit"
)

func NewRunner(
	logger lager.Logger,
	clock clock.Clock,
	interval time.Duration,
	reapFunc func(lager.Logger) error,
) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)

		ticker := clock.NewTicker(interval)
		defer ticker.Stop()

		for {
			tickLog := logger.Session("tick")

			select {
			case <-ticker.C():
				err := reapFunc(tickLog)
				if err != nil {
					tickLog.Error("failed-to-reap", err)
				}

			case <-signals:
				return nil
			}
		}
	})
}
