package uidgid

import (
	"os"
	"os/exec"
	"syscall"
)

func (t *translator) getuidgid(info os.FileInfo) (int, int, error) {
	return int(info.Sys().(*syscall.Stat_t).Uid), int(info.Sys().(*syscall.Stat_t).Gid), nil
}

func (t *translator) setuidgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
		Credential: &syscall.Credential{
			Uid: 0,
			Gid: 0,
		},
		GidMappingsEnableSetgroups: true,
	}

	t.mapper.Apply(cmd)
}
