package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/concourse/baggageclaim/volume"
)

type OverlayDriver struct {
	VolumesDir  string
	OverlaysDir string
}

func NewOverlayDriver(volumesDir, overlaysDir string) (volume.Driver, error) {
	driver := &OverlayDriver{
		VolumesDir:  volumesDir,
		OverlaysDir: overlaysDir,
	}

	err := driver.RecoverMountTable(filepath.Join(volumesDir, "live"))
	if err != nil {
		return nil, err
	}
	return driver, nil
}

func (driver *OverlayDriver) CreateVolume(path string) error {
	layerDir := driver.layerDir(path)

	err := os.MkdirAll(layerDir, 0755)
	if err != nil {
		return err
	}

	err = os.Mkdir(path, 0755)
	if err != nil {
		return err
	}

	return syscall.Mount(layerDir, path, "", syscall.MS_BIND, "")
}

func (driver *OverlayDriver) DestroyVolume(path string) error {
	err := syscall.Unmount(path, 0)
	// when a path is already unmounted, and unmount is called
	// on it, syscall.EINVAL is returned as an error
	// ignore this error and continue to clean up
	if err != nil && err != os.ErrInvalid {
		return err
	}

	err = os.RemoveAll(driver.workDir(path))
	if err != nil {
		return err
	}

	err = os.RemoveAll(driver.layerDir(path))
	if err != nil {
		return err
	}

	return os.RemoveAll(path)
}

func (driver *OverlayDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	childDir := driver.layerDir(path)
	workDir := driver.workDir(path)

	err := os.MkdirAll(childDir, 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(workDir, 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	opts := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		parent,
		childDir,
		workDir,
	)

	return syscall.Mount("overlay", path, "overlay", 0, opts)
}

// We apply 2 types of mounts when using the overlay driver
//
// 1. For any new volume, we create its root in the overlaysDir and
// 	  issue a bind mount of that root into the liveVolumesDir
// 2. For COW volumes, we create upper and work dirs and use
// 	  ancestry to determine the lower dir. These dirs are
//	  used to create an overlay mount.
//
// These mounts can disappear when the system reboots (mount table cleared)
// As a precaution we reattach mounts during startup to fix missing ones
func (driver *OverlayDriver) RecoverMountTable(liveVolumesDir string) error {
	// skip recovery if live dir hasn't been initialized (nothing to mount)
	if _, err := os.Stat(liveVolumesDir); os.IsNotExist(err) {
		return nil
	}

	liveVolumes, err := ioutil.ReadDir(liveVolumesDir)
	if err != nil {
		return err
	}

	for _, volumeFileInfo := range liveVolumes {

		liveVolumePath := filepath.Join(liveVolumesDir, volumeFileInfo.Name())
		liveVolumeDataPath := filepath.Join(liveVolumePath, "volume")
		parentSymlink := filepath.Join(liveVolumePath, "parent")

		// a parent symlink indicates a COW
		if _, err := os.Stat(parentSymlink); err == nil {
			parentPath, err := os.Readlink(parentSymlink)
			if err != nil {
				return err
			}
			parentDataPath := filepath.Join(parentPath, "volume")

			ancestry, err := driver.ancestry(parentDataPath)
			if err != nil {
				return err
			}

			err = driver.applyOverlayMount(ancestry, liveVolumeDataPath)
			if err != nil {
				return err
			}
		} else if os.IsNotExist(err) {
			err := driver.applyBindMount(liveVolumeDataPath)
			if err != nil {
				return err
			}
		}

		if err != nil {
			return err
		}

	}

	return nil
}

func (driver *OverlayDriver) applyBindMount(datapath string) error {
	layerDir := driver.layerDir(datapath)

	err := syscall.Mount(layerDir, datapath, "", syscall.MS_BIND, "")
	if err != nil {
		return err
	}

	return nil
}

func (driver *OverlayDriver) applyOverlayMount(ancestry []string, datapath string) error {
	opts := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(ancestry, ":"),
		driver.layerDir(datapath),
		driver.workDir(datapath),
	)

	err := syscall.Mount("overlay", datapath, "overlay", 0, opts)
	if err != nil {
		return err
	}

	return nil
}

func (driver *OverlayDriver) layerDir(datapath string) string {
	return filepath.Join(driver.OverlaysDir, driver.getGUID(datapath))
}

func (driver *OverlayDriver) workDir(datapath string) string {
	return filepath.Join(driver.OverlaysDir, "work", driver.getGUID(datapath))
}

func (driver *OverlayDriver) ancestry(datapath string) ([]string, error) {
	ancestry := []string{}

	currentPath := datapath
	for {
		ancestry = append(ancestry, driver.layerDir(currentPath))

		parentVolume, err := os.Readlink(filepath.Join(filepath.Dir(currentPath), "parent"))
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				break
			}

			return nil, err
		}

		currentPath = filepath.Join(parentVolume, "volume")
	}

	return ancestry, nil
}

func (driver *OverlayDriver) getGUID(datapath string) string {
	return filepath.Base(filepath.Dir(datapath))
}
