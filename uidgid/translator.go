package uidgid

import (
	"fmt"
	"os"
	"os/exec"
)

//go:generate counterfeiter . Translator

type Translator interface {
	TranslatePath(path string, info os.FileInfo, err error) error
	TranslateCommand(*exec.Cmd)
}

type translator struct {
	mappings StringMapper
	chown    func(path string, uid int, gid int) error
}

type Mapper interface {
	Map(int, int) (int, int)
	Apply(*exec.Cmd)
}

type StringMapper interface {
	fmt.Stringer
	Mapper
}

func NewTranslator(maxID int) *translator {
	return &translator{
		mappings: newMappings(maxID),
		chown:    os.Lchown,
	}
}

func (t *translator) CacheKey() string {
	return t.mappings.String()
}

func (t *translator) TranslatePath(path string, info os.FileInfo, err error) error {
	uid, gid, _ := t.getuidgid(info)

	touid, togid := t.mappings.Map(uid, gid)

	if touid != uid || togid != gid {
		t.chown(path, touid, togid)
	}

	return nil
}

func (t *translator) TranslateCommand(cmd *exec.Cmd) {
	t.setuidgid(cmd)
}
