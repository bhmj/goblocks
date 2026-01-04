package clock

import "time"

type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type FakeClock struct {
	now time.Time
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

func (f *FakeClock) Now() time.Time {
	return f.now
}

func (f *FakeClock) Sleep(d time.Duration) {
	f.now = f.now.Add(d)
}

type RealClock struct{}

func NewClock() *RealClock {
	return &RealClock{}
}

func (RealClock) Now() time.Time {
	return time.Now()
}

func (RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}
