package fs

import (
	"fmt"
	"os"
	"os/exec"
)

func CreateBtrFSVolume(diskImagePath string, loopbackDevicePath string, mountPath string) error {
	createDiskImage := exec.Command(
		"dd",
		"if=/dev/zero",
		fmt.Sprintf("of=%s", diskImagePath),
		"bs=1024",
		"count=307200",
	)
	createDiskImage.Stdout = os.Stdout
	createDiskImage.Stderr = os.Stderr
	if err := createDiskImage.Run(); err != nil {
		return err
	}

	assignLoopbackDevice := exec.Command(
		"losetup",
		loopbackDevicePath,
		diskImagePath,
	)
	assignLoopbackDevice.Stdout = os.Stdout
	assignLoopbackDevice.Stderr = os.Stderr
	if err := assignLoopbackDevice.Run(); err != nil {
		return err
	}

	createFsOnDevice := exec.Command(
		"mkfs.btrfs",
		loopbackDevicePath,
	)
	createFsOnDevice.Stdout = os.Stdout
	createFsOnDevice.Stderr = os.Stderr
	if err := createFsOnDevice.Run(); err != nil {
		return err
	}

	if err := os.MkdirAll(mountPath, 0755); err != nil {
		return err
	}

	mountFs := exec.Command(
		"mount",
		loopbackDevicePath,
		mountPath,
	)
	mountFs.Stdout = os.Stdout
	mountFs.Stderr = os.Stderr
	if err := mountFs.Run(); err != nil {
		return err
	}

	return nil
}

func DeleteBtrFSVolume(diskImagePath string, loopbackDevicePath string, mountPath string) error {
	unmountFs := exec.Command(
		"umount",
		mountPath,
	)
	unmountFs.Stdout = os.Stdout
	unmountFs.Stderr = os.Stderr
	if err := unmountFs.Run(); err != nil {
		return err
	}

	if err := os.RemoveAll(mountPath); err != nil {
		return err
	}

	removeFsFromDevice := exec.Command(
		"losetup",
		"-d",
		loopbackDevicePath,
	)
	removeFsFromDevice.Stdout = os.Stdout
	removeFsFromDevice.Stderr = os.Stderr
	if err := removeFsFromDevice.Run(); err != nil {
		return err
	}

	return os.Remove(diskImagePath)
}
