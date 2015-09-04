package volume

type Locker interface {
	Lock(handle string) error
	Unlock(handle string) error
}

func NewLocker() Locker {
	return &locker{}
}

type locker struct {
}

func (l *locker) Lock(handle string) error {
	return nil
}

func (l *locker) Unlock(handle string) error {
	return nil
}
