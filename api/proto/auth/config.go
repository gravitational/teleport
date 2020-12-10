/*
Copyright 2020 Gravitational, Inc.

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

package auth

import (
	"crypto/tls"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api"
	"github.com/gravitational/trace"
)

// Config contains configuration of the client
type Config struct {
	// Addrs is a list of teleport auth/proxy server addresses to dial
	Addrs []string
	// Dialer is a custom dialer, if provided
	// is used instead of the list of addresses
	Dialer ContextDialer
	// DialTimeout defines how long to attempt dialing before timing out
	DialTimeout time.Duration
	// KeepAlivePeriod defines period between keep alives
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies amount of missed keep alives
	// to wait for until declaring connection as broken
	KeepAliveCount int
	// TLS is a TLS config
	TLS *tls.Config
}

// CheckAndSetDefaults checks and sets default config values
func (c *Config) CheckAndSetDefaults() error {
	if len(c.Addrs) == 0 && c.Dialer == nil {
		return trace.BadParameter("set parameter Addrs or Dialer")
	}
	if len(c.Addrs) != 0 && c.Dialer != nil {
		return trace.BadParameter("set parameter Addrs or Dialer, not both")
	}
	if c.TLS == nil {
		return trace.BadParameter("missing parameter TLS")
	}
	if c.KeepAlivePeriod == 0 {
		c.KeepAlivePeriod = api.ServerKeepAliveTTL
	}
	if c.KeepAliveCount == 0 {
		c.KeepAliveCount = api.KeepAliveCountMax
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = api.DefaultDialTimeout
	}
	if c.Dialer == nil {
		var err error
		if c.Dialer, err = NewAddrDialer(c.Addrs, c.KeepAlivePeriod, c.DialTimeout); err != nil {
			return err
		}
	}
	if c.TLS.ServerName == "" {
		c.TLS.ServerName = teleport.APIDomain
	}

	return nil
}
