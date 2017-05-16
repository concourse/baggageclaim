package volume

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim/uidgid"
)

type NamespacedStrategy struct {
	PreStrategy Strategy

	Namespacer uidgid.Namespacer
}

func (strategy NamespacedStrategy) Materialize(logger lager.Logger, handle string, fs Filesystem) (FilesystemInitVolume, error) {
	volume, err := strategy.PreStrategy.Materialize(logger, handle, fs)
	if err != nil {
		return nil, err
	}

	err = strategy.Namespacer.NamespacePath(logger, volume.DataPath())
	if err != nil {
		volume.Destroy()
		return nil, err
	}

	return volume, nil
}
