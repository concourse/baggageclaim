package volume

import (
	"fmt"

	"github.com/otiai10/copy"
)

func cp(followSymlinks bool, src, dest string) error {
	if followSymlinks {
		return fmt.Errorf("FollowSymlinks not supported on Windows")
	}

	return copy.Copy(src, dest)
}
