// +build !linux

package uidjunk

import "os"

func getuidgid(info os.FileInfo) (int, int, error) {
	panic("not supported")
}
