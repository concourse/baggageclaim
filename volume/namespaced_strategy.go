package volume

import "github.com/concourse/baggageclaim/uidjunk"

type NamespacedStrategy struct {
	PreStrategy Strategy

	Namespacer uidjunk.Namespacer
}

func (strategy NamespacedStrategy) Materialize(handle string, fs Filesystem) (FilesystemInitVolume, error) {
	volume, err := strategy.PreStrategy.Materialize(handle, fs)
	if err != nil {
		return nil, err
	}

	err = strategy.Namespacer.Namespace(volume.DataPath())
	if err != nil {
		volume.Destroy()
		return nil, err
	}

	return volume, nil
}
