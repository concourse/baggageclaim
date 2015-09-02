package bomberman

import (
	"github.com/concourse/baggageclaim/bomberman/timebomb"
	"github.com/concourse/baggageclaim/volume"
)

type Bomberman struct {
	repository volume.Repository

	detonate func(handle string)

	strap   chan volume.Volume
	pause   chan string
	unpause chan string
	defuse  chan string
	cleanup chan string
}

func New(repository volume.Repository, detonate func(handle string)) *Bomberman {
	b := &Bomberman{
		repository: repository,
		detonate:   detonate,

		strap:   make(chan volume.Volume),
		pause:   make(chan string),
		unpause: make(chan string),
		defuse:  make(chan string),
		cleanup: make(chan string),
	}

	go b.manageBombs()

	return b
}

func (b *Bomberman) Strap(volume volume.Volume) {
	b.strap <- volume
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
		case volume := <-b.strap:
			if b.repository.TTL(volume) == 0 {
				continue
			}

			bomb := timebomb.New(
				b.repository.TTL(volume),
				func() {
					b.detonate(volume.GUID)
					b.cleanup <- volume.GUID
				},
			)

			timeBombs[volume.GUID] = bomb

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
