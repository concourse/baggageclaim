package volume

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/uidjunk"
)

type Strategerizer interface {
	StrategyFor(baggageclaim.VolumeRequest) (Strategy, error)
}

const (
	StrategyEmpty       = "empty"
	StrategyCopyOnWrite = "cow"
	StrategyImport      = "import"
)

var ErrNoStrategy = errors.New("no strategy given")
var ErrUnknownStrategy = errors.New("unknown strategy")

type strategerizer struct {
	namespacer uidjunk.Namespacer
}

func NewStrategerizer(namespacer uidjunk.Namespacer) Strategerizer {
	return &strategerizer{
		namespacer: namespacer,
	}
}

func (s *strategerizer) StrategyFor(request baggageclaim.VolumeRequest) (Strategy, error) {
	if request.Strategy == nil {
		return nil, ErrNoStrategy
	}

	var strategyInfo map[string]string
	err := json.Unmarshal(*request.Strategy, &strategyInfo)
	if err != nil {
		return nil, fmt.Errorf("malformed strategy: %s", err)
	}

	var strategy Strategy
	switch strategyInfo["type"] {
	case StrategyEmpty:
		strategy = EmptyStrategy{}
	case StrategyCopyOnWrite:
		strategy = COWStrategy{strategyInfo["volume"]}
	case StrategyImport:
		strategy = ImportStrategy{strategyInfo["path"]}
	default:
		return nil, ErrUnknownStrategy
	}

	if !request.Privileged {
		strategy = NamespacedStrategy{
			PreStrategy: strategy,
			Namespacer:  s.namespacer,
		}
	}

	return strategy, nil
}
