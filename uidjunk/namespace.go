package uidjunk

import (
	"os"
	"os/exec"
	"path/filepath"

	"code.cloudfoundry.org/lager"
)

//go:generate counterfeiter -o fake_namespacer/fake_namespacer.go . Namespacer
type Namespacer interface {
	CacheKey() string
	NamespacePath(rootfsPath string) error
	NamespaceCommand(cmd *exec.Cmd)
}

//go:generate counterfeiter -o fake_translator/fake_translator.go . Translator
type Translator interface {
	CacheKey() string
	TranslatePath(path string, info os.FileInfo, err error) error
	TranslateCommand(exec.Cmd) exec.Cmd
}

type UidNamespacer struct {
	Translator Translator
	Logger     lager.Logger
}

func (n *UidNamespacer) NamespacePath(rootfsPath string) error {
	log := n.Logger.Session("namespace-rootfs", lager.Data{
		"path": rootfsPath,
	})

	log.Info("namespace")

	if err := filepath.Walk(rootfsPath, n.Translator.TranslatePath); err != nil {
		log.Error("walk-failed", err)
	}

	log.Info("namespaced")

	return nil
}

func (n *UidNamespacer) NamespaceCommand(cmd *exec.Cmd) {
	n.Translator.TranslateCommand(cmd)
}

func (n *UidNamespacer) CacheKey() string {
	return n.Translator.CacheKey()
}

type NoopNamespacer struct{}

func (NoopNamespacer) NamespacePath(string) error     { return nil }
func (NoopNamespacer) NamespaceCommand(cmd *exec.Cmd) {}
func (NoopNamespacer) CacheKey() string               { return "" }
