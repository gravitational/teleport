/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sshutils

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
)

// CheckersGetter defines a function that returns a list of ssh public keys.
type CheckersGetter func() ([]ssh.PublicKey, error)

// HostKeyCallbackConfig is the host key callback configuration.
type HostKeyCallbackConfig struct {
	// GetHostCheckers is used to fetch host checking (public) keys.
	GetHostCheckers CheckersGetter
	// HostKeyFallback sets optional callback to check non-certificate keys.
	HostKeyFallback ssh.HostKeyCallback
	// FIPS allows to set FIPS mode which will validate algorithms.
	FIPS bool
	// OnCheckCert is called on SSH certificate validation.
	OnCheckCert func(*ssh.Certificate) error
	// Clock is used to set the Checker Time
	Clock clockwork.Clock
}

// Check validates the config.
func (c *HostKeyCallbackConfig) Check() error {
	if c.GetHostCheckers == nil {
		return trace.BadParameter("missing GetHostCheckers")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewHostKeyCallback returns host key callback function with the specified parameters.
func NewHostKeyCallback(conf HostKeyCallbackConfig) (ssh.HostKeyCallback, error) {
	if err := conf.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	checker := CertChecker{
		CertChecker: ssh.CertChecker{
			IsHostAuthority: makeIsHostAuthorityFunc(conf.GetHostCheckers),
			HostKeyFallback: conf.HostKeyFallback,
			Clock:           conf.Clock.Now,
		},
		FIPS:        conf.FIPS,
		OnCheckCert: conf.OnCheckCert,
	}
	return checker.CheckHostKey, nil
}

func makeIsHostAuthorityFunc(getCheckers CheckersGetter) func(authority ssh.PublicKey, host string) bool {
	return func(authority ssh.PublicKey, host string) bool {
		checkers, err := getCheckers()
		if err != nil {
			slog.ErrorContext(context.Background(), "Failed to get checkers.", "host", host, "error", err)
			return false
		}
		for _, checker := range checkers {
			if KeysEqual(authority, checker) {
				return true
			}
		}
		slog.DebugContext(context.Background(), "No CA found for target host.", "host", host)
		return false
	}
}
