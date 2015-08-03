package utils

import (
	"time"
)

type Clock interface {
	Now() time.Time
}

type TestClock struct {
	N time.Time
}

func (t *TestClock) Now() time.Time {
	return t.N
}

func (t *TestClock) Advance(d time.Duration) {
	t.N = t.N.Add(d)
}

type WallClock struct {
}

func (*WallClock) Now() time.Time {
	return time.Now()
}

var RealTime = &WallClock{}
