package driver

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type OverlayDriver struct {
	OverlaysDir string
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

	return syscall.Mount(path, layerDir, "", syscall.MS_BIND, "")
}

func (driver *OverlayDriver) DestroyVolume(path string) error {
	err := syscall.Unmount(path, 0)
	if err != nil {
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
	ancestry, err := driver.ancestry(parent)
	if err != nil {
		return err
	}

	childDir := driver.layerDir(path)
	workDir := driver.workDir(path)

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
		strings.Join(ancestry, ":"),
		childDir,
		workDir,
	)

	return syscall.Mount("overlay", path, "overlay", 0, opts)
}

func (driver *OverlayDriver) GetVolumeSizeInBytes(path string) (int64, error) {
	stdout := &bytes.Buffer{}
	cmd := exec.Command("du", driver.layerDir(path))
	cmd.Stdout = stdout

	err := cmd.Run()
	if err != nil {
		return 0, err
	}

	var size int64
	_, err = fmt.Sscanf(stdout.String(), "%d", &size)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func (driver *OverlayDriver) layerDir(path string) string {
	return filepath.Join(driver.OverlaysDir, driver.pathId(path))
}

func (driver *OverlayDriver) workDir(path string) string {
	return filepath.Join(driver.OverlaysDir, "work", driver.pathId(path))
}

func (driver *OverlayDriver) ancestry(path string) ([]string, error) {
	ancestry := []string{}

	currentPath := path
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

func (driver *OverlayDriver) pathId(path string) string {
	return filepath.Base(filepath.Dir(path))
}
