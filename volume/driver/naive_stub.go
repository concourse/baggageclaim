// +build !windows

package driver

import (
	"os/exec"
)

func (driver *NaiveDriver) CreateCopyOnWriteLayer(path string, parent string) error {
	return exec.Command("cp", "-rp", parent, path).Run()
}
