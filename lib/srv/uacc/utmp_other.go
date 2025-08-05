//go:build !linux

package uacc

import (
	"net"
	"time"

	"github.com/gravitational/trace"
)

type UtmpBackend struct{}

func NewUtmpBackend(utmpFile, wtmpFile, btmpFile string) (*UtmpBackend, error) {
	return nil, trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) Login(_, _ string, _ net.Addr, _ time.Time) (string, error) {
	return "", trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) Logout(_ string, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) FailedLogin(_ string, _ net.Addr, _ time.Time) error {
	return trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) IsUserInFile(_ string, _ string) (bool, error) {
	return false, trace.NotImplemented("utmp is linux only")
}

func (u *UtmpBackend) IsUserLoggedIn(_ string) (bool, error) {
	return false, trace.NotImplemented("utmp is linux only")
}
