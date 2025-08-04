//go:build !linux

package uacc

import (
	"net"
	"time"

	"github.com/gravitational/trace"
)

type utmpBackend struct{}

func newUtmpBackend(utmpFile, wtmpFile, btmpFile string) (*utmpBackend, error) {
	return nil, trace.NotImplemented("utmp is linux only")
}

func (u *utmpBackend) Name() string {
	return "utmp"
}

func (u *utmpBackend) Login(_, _ string, _ net.Addr, _ time.Time) (string, error) {
	return "", trace.NotImplemented("utmp is linux only")
}

func (u *utmpBackend) Logout(_ string, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *utmpBackend) FailedLogin(_ string, _ net.Addr, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *utmpBackend) IsUserLoggedIn(_ string) (bool, error) {
	return false, trace.NotImplemented("utmp is linux only")
}
