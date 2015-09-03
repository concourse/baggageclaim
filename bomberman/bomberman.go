package bomberman

import (
	"github.com/concourse/baggageclaim/bomberman/timebomb"
	"github.com/concourse/baggageclaim/volume"
)

type Bomberman struct {
	detonate func(handle string)

	strap   chan volume.Volume
	pause   chan string
	unpause chan string
	defuse  chan string
	cleanup chan string
}

func New(detonate func(handle string)) *Bomberman {
	b := &Bomberman{
		detonate: detonate,

		strap:   make(chan volume.Volume),
		pause:   make(chan string),
		unpause: make(chan string),
		defuse:  make(chan string),
		cleanup: make(chan string),
	}

	go b.manageBombs()

	return b
}

func (b *Bomberman) Strap(vol volume.Volume) {
	b.strap <- vol
}

func (b *Bomberman) Pause(name string) {
	b.pause <- name
}

func (b *Bomberman) Unpause(name string) {
	b.unpause <- name
}

func (b *Bomberman) Defuse(name string) {
	b.defuse <- name
}

func (b *Bomberman) manageBombs() {
	timeBombs := map[string]*timebomb.TimeBomb{}

	for {
		select {
		case vol := <-b.strap:
			if vol.TTL.Duration() == 0 {
				continue
			}

			bomb := timebomb.New(
				vol.TTL.Duration(),
				func() {
					b.detonate(vol.Handle)
					b.cleanup <- vol.Handle
				},
			)

			timeBombs[vol.Handle] = bomb

			bomb.Strap()

		case handle := <-b.pause:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Pause()

		case handle := <-b.unpause:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Unpause()

		case handle := <-b.defuse:
			bomb, found := timeBombs[handle]
			if !found {
				continue
			}

			bomb.Defuse()

			delete(timeBombs, handle)

		case handle := <-b.cleanup:
			delete(timeBombs, handle)
		}
	}
}
