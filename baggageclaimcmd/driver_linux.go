package baggageclaimcmd

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
)

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) volume.Driver {
	switch cmd.Driver {
	case "overlay":
		return &driver.OverlayDriver{
			OverlaysDir: cmd.OverlaysDir,
		}
	case "btrfs":
		return driver.NewBtrFSDriver(
			logger.Session("driver"),
			string(cmd.VolumesDir),
			cmd.BtrfsBin,
		)
	}

	return &driver.NaiveDriver{}
}
