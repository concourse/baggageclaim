package driver

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"

	"github.com/concourse/baggageclaim/volume"
	"github.com/concourse/baggageclaim/volume/copy"
)

//go:generate counterfeiter . Mounter

type LiveVolume struct {
	Path   string
	Parent *LiveVolume
}

type Mounter interface {
	BindMount(datapath string) error
	OverlayMount(parent, datapath string) error
}

type OverlayDriver struct {
	VolumesDir  string
	OverlaysDir string
}

func NewOverlayDriver(volumesDir, overlaysDir string) (volume.Driver, error) {
	driver := &OverlayDriver{
		VolumesDir:  volumesDir,
		OverlaysDir: overlaysDir,
	}

	err := RecoverMountTable(filepath.Join(volumesDir, "live"), driver)
	if err != nil {
		err = fmt.Errorf("recover mount table: %w", err)
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

	parentGUID := driver.getGUID(parent)
	root := filepath.Dir(filepath.Dir(parent))
	parentVol, err := NewLiveVolume(root, parentGUID)
	if err != nil {
		return err
	}

	if parentVol.Parent != nil {
		parentDir := driver.layerDir(parent)
		err := copy.Cp(false, parentDir, childDir)
		if err != nil {
			return fmt.Errorf("copy parent data to child: %w", err)
		}

		parent = filepath.Join(parentVol.Parent.Path, "volume")
	}

	err = os.MkdirAll(childDir, 0755)
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

func (driver *OverlayDriver) BindMount(datapath string) error {
	layerDir := driver.layerDir(datapath)

	err := syscall.Mount(layerDir, datapath, "", syscall.MS_BIND, "")
	if err != nil {
		return err
	}

	return nil
}

func (driver *OverlayDriver) OverlayMount(mountpoint, parent string) error {
	opts := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		parent,
		driver.layerDir(mountpoint),
		driver.workDir(mountpoint),
	)

	err := syscall.Mount("overlay", mountpoint, "overlay", 0, opts)
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

func (driver *OverlayDriver) getGUID(datapath string) string {
	return filepath.Base(filepath.Dir(datapath))
}

// NewLiveVolume creates the representation of a volume under the `live`
// directory.
//
// It captures the filepath to the volume, as well as its parents (in a singly
// linked list).
//
// ps.: it DOES NOT protect itself against circular dependencies.
//
func NewLiveVolume(root, vol string) (*LiveVolume, error) {
	volDir := filepath.Join(root, vol)
	parentSymlink := filepath.Join(volDir, "parent")
	liveVol := &LiveVolume{Path: volDir}

	_, err := os.Stat(volDir)
	if err != nil {
		return nil, err
	}

	parentDir, err := readlink(parentSymlink)
	if err != nil {
		if os.IsNotExist(err) {
			return liveVol, nil
		}

		return nil, err
	}

	liveVol.Parent, err = NewLiveVolume(root, filepath.Base(parentDir))
	if err != nil {
		return nil, err
	}

	return liveVol, nil
}

// Ancestry traverses LiveVolume linked list producing a list of LiveVolumes
// from "no dependencies" (oldest) to "with dependencies" (youngest).
//
func Ancestry(vol LiveVolume) []LiveVolume {
	res := []LiveVolume{}

	for current := &vol; current != nil; current = current.Parent {
		res = append([]LiveVolume{*current}, res...)
	}

	return res
}

// RecoverMountTable takes care of mounting volumes that exist under `live`, but
// are not yet mounted due to a system shutdown.
//
// It takes care of finding out the dependencies between volumes (represented by
// symbolic links at `live/<vol>/parent`), and making sure that:
// - dependencies are mounted first
// - mountpoints are not mounted more than once
//
//  e.g, given the following `live` dir:
//
//		.
//		└── live
//		    ├── vol1
//		    ├── vol2
//		    │   └── parent -> ./vol3
//		    ├── vol3
//		    │   └── parent -> ./vol1
//		    └── vol4
//			└── parent -> ./vol1
//
//
// it figures out an acceptable mounting order is `vol1`, `vol3`, `vol2`,
// `vol4`, issuing precisely only 4 `mount`s.
//
func RecoverMountTable(root string, mounter Mounter) error {
	dirs, err := ioutil.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		err = fmt.Errorf("readdir %s: %w", root, err)
		return err
	}

	table := NewMountTable(mounter)

	for _, dir := range dirs {
		liveVol, err := NewLiveVolume(root, dir.Name())
		if err != nil {
			return err
		}

		ancestry := Ancestry(*liveVol)
		for _, vol := range ancestry {
			err = table.Mount(vol)
			if err != nil {
				return fmt.Errorf("table mount %+v: %w", vol, err)
			}
		}
	}

	return nil
}

// mountTable is responsible for keeping track of places that have already been
// mounted, as well as applying the mounts when neeeded.
//
type mountTable struct {
	mounter Mounter
	mounted map[string]interface{}
}

func NewMountTable(mounter Mounter) mountTable {
	return mountTable{
		mounter: mounter,
		mounted: make(map[string]interface{}),
	}
}

func (m mountTable) isAlreadyMounted(vol LiveVolume) bool {
	_, found := m.mounted[vol.Path]
	return found
}

func (m mountTable) markMounted(vol LiveVolume) {
	m.mounted[vol.Path] = nil
}

// Mount mounts a live volume if necessary.
//
func (m mountTable) Mount(vol LiveVolume) error {
	var (
		err      error
		datapath = filepath.Join(vol.Path, "volume")
	)

	if m.isAlreadyMounted(vol) {
		return nil
	}

	if vol.Parent == nil {
		err = m.mounter.BindMount(datapath)
		if err != nil {
			return err
		}

		m.markMounted(vol)
		return nil
	}

	parentDatapath := filepath.Join(vol.Parent.Path, "volume")
	err = m.mounter.OverlayMount(datapath, parentDatapath)
	if err != nil {
		return err
	}

	m.markMounted(vol)
	return nil
}

func readlink(path string) (target string, err error) {
	finfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}

	if finfo.Mode()&os.ModeSymlink != os.ModeSymlink {
		return "", fmt.Errorf("not a symlink")
	}

	target, err = os.Readlink(path)
	if err != nil {
		return "", err
	}

	return target, nil
}
