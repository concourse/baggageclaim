package volume

import "github.com/pivotal-golang/lager"

//go:generate counterfeiter . Strategy

type Strategy interface {
	Materialize(lager.Logger, string, Filesystem) (FilesystemInitVolume, error)
}
