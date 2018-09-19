//+build !windows

package volume

import (
	"os/exec"
)

func cp(followSymlinks bool, src, dest string) error {
	cpFlags := "-a"
	if followSymlinks {
		cpFlags = "-Lr"
	}

	return exec.Command("cp", cpFlags, src+"/.", dest).Run()
}
