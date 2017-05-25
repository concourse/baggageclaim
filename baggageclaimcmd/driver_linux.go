package baggageclaimcmd

import (
	"errors"
	"fmt"
	"syscall"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/fs"
	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/driver"
)

func (cmd *BaggageclaimCommand) driver(logger lager.Logger) (volume.Driver, error) {
	var fsStat syscall.Statfs_t
	err = syscall.Statfs(volumesDir, &fsStat)
	if err != nil {
		return nil, fmt.Errorf("failed to stat volumes filesystem: %s", err)
	}

	kernelSupportsOverlay := kernel.CheckKernelVersion(4, 0, 0)

	if cmd.Driver == "detect" {
		if fsStat.Type == btrfsFSType {
			cmd.Driver = "btrfs"
		} else if kernelSupportsOverlay {
			cmd.Driver = "overlay"
		} else {
			cmd.Driver = "naive"
		}
	}

	volumesDir := cmd.VolumesDir.Path()

	if cmd.Driver == "btrfs" && fsStat.Type != btrfsFSType {
		volumesImage := volumesDir + ".img"
		filesystem := fs.New(logger.Session("fs"), volumesImage, volumesDir, cmd.MkfsBin)

		diskSize := fsStat.Blocks * uint64(fsStat.Bsize)
		mountSize := diskSize - (10 * 1024 * 1024 * 1024)
		if mountSize < 0 {
			mountSize = diskSize
		}

		err = filesystem.Create(mountSize)
		if err != nil {
			return nil, fmt.Errorf("failed to create btrfs filesystem: %s", err)
		}
	}

	if cmd.Driver == "overlay" && !kernelSupportsOverlay {
		return nil, errors.New("overlay driver requires kernel version >= 4.0.0")
	}

	switch cmd.Driver {
	case "overlay":
		return &driver.OverlayDriver{
			OverlaysDir: cmd.OverlaysDir,
		}
	case "btrfs":
		return driver.NewBtrFSDriver(logger.Session("driver"), cmd.BtrfsBin)
	}

	return &driver.NaiveDriver{}, nil
}
