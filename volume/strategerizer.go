package volume

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/concourse/baggageclaim"
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

type strategerizer struct{}

func NewStrategerizer() Strategerizer {
	return &strategerizer{}
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

	return strategy, nil
}
