package uidjunk

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type UidTranslator struct {
	maxID int

	getuidgid func(os.FileInfo) (int, int, error)
	chown     func(path string, uid, gid int) error
}

type Mapper interface {
	Map(id int) int
}

type StringMapper interface {
	fmt.Stringer
	Mapper
}

func NewUidTranslator(maxID int) *UidTranslator {
	return &UidTranslator{
		maxID: maxID,

		getuidgid: getuidgid,
		chown:     os.Lchown,
	}
}

func (u UidTranslator) TranslatePath(path string, info os.FileInfo, err error) error {
	uid, gid, _ := u.getuidgid(info)
	touid, togid := u.uidMappings.Map(uid), u.gidMappings.Map(gid)

	if touid != uid || togid != gid {
		u.chown(path, touid, togid)
	}

	return nil
}

func (u UidTranslator) TranslateCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 syscall.CLONE_NEWUSER,
		UidMappings:                u.uidMappings,
		GidMappings:                u.gidMappings,
		GidMappingsEnableSetgroups: true,
	}
}

func (u UidTranslator) CacheKey() string {
	return fmt.Sprintf("%s+%s", u.uidMappings.String(), u.gidMappings.String())
}
