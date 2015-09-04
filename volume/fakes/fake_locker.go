package fakes

import (
	"sync/atomic"

	"github.com/concourse/baggageclaim/volume"
)

type Locker struct {
	realLocker volume.Locker
	lockChan   chan interface{}
	lockedChan chan interface{}
	unlockChan chan interface{}

	lockCallCount   uint64
	unlockCallCount uint64
}

func NewLocker(
	l volume.Locker,
	lockChan chan interface{},
	lockedChan chan interface{},
	unlockChan chan interface{},
) *Locker {
	return &Locker{
		realLocker:      l,
		lockChan:        lockChan,
		lockedChan:      lockedChan,
		unlockChan:      unlockChan,
		lockCallCount:   0,
		unlockCallCount: 0,
	}
}

func (l *Locker) Lock(handle string) error {
	atomic.AddUint64(&l.lockCallCount, 1)
	<-l.lockChan
	l.lockedChan <- nil
	return nil
}

func (l *Locker) Unlock(handle string) error {
	atomic.AddUint64(&l.unlockCallCount, 1)
	<-l.unlockChan
	l.lockChan <- nil
	return nil
}

func (l *Locker) LockCallCount() int {
	return int(atomic.LoadUint64(&l.lockCallCount))
}

func (l *Locker) UnlockCallCount() int {
	return int(atomic.LoadUint64(&l.unlockCallCount))
}
