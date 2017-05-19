package uidgid

import (
	"os"
	"os/exec"
)

//go:generate counterfeiter . Translator

type Translator interface {
	TranslatePath(path string, info os.FileInfo, err error) error
	TranslateCommand(*exec.Cmd)
}

type translator struct {
	mapper Mapper
	chown  func(path string, uid int, gid int) error
}

type Mapper interface {
	Map(int, int) (int, int)
	Apply(*exec.Cmd)
}

func NewTranslator(mapper Mapper) *translator {
	return &translator{
		mapper: mapper,
		chown:  os.Lchown,
	}
}

func (t *translator) TranslatePath(path string, info os.FileInfo, err error) error {
	uid, gid, _ := t.getuidgid(info)

	touid, togid := t.mapper.Map(uid, gid)

	if touid != uid || togid != gid {
		t.chown(path, touid, togid)
	}

	return nil
}

func (t *translator) TranslateCommand(cmd *exec.Cmd) {
	t.setuidgid(cmd)
}
