/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package config

import (
	"crypto/tls"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

const UnstableClientCredentialOutputType = "unstable_client_credential"

var (
	_ ServiceConfig      = &UnstableClientCredentialOutput{}
	_ client.Credentials = &UnstableClientCredentialOutput{}
)

// UnstableClientCredentialOutput is an experimental tbot output which is
// compatible with the client.Credential interface. This allows tbot to be
// used as an in-memory source of credentials for the Teleport API client and
// removes the need to write credentials to a filesystem.
//
// Unstable: no API stability promises are made for this struct and its methods.
// Available configuration options may change and the signatures of methods may
// be modified. This output is currently part of an experiment and could be
// removed in a future release.
type UnstableClientCredentialOutput struct {
	mu     sync.Mutex
	facade *identity.Facade
	ready  chan struct{}
}

// Ready returns a channel which closes when the Output is ready to be used
// as a client credential. Using this as a credential before Ready closes is
// unsupported.
func (o *UnstableClientCredentialOutput) Ready() <-chan struct{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ready == nil {
		o.ready = make(chan struct{})
		if o.facade != nil {
			close(o.ready)
		}
	}
	return o.ready
}

// Dialer implements the client.Credential interface. It does nothing.
func (o *UnstableClientCredentialOutput) Dialer(c client.Config) (client.ContextDialer, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return nil, trace.NotImplemented("no dialer")
}

// TLSConfig implements the client.Credential interface and return the
// tls.Config from the underlying identity.Facade.
func (o *UnstableClientCredentialOutput) TLSConfig() (*tls.Config, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.facade == nil {
		return nil, trace.BadParameter("credentials not yet ready")
	}
	return o.facade.TLSConfig()
}

// SSHClientConfig implements the client.Credential interface and return the
// ssh.ClientConfig from the underlying identity.Facade.
func (o *UnstableClientCredentialOutput) SSHClientConfig() (*ssh.ClientConfig, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.facade == nil {
		return nil, trace.BadParameter("credentials not yet ready")
	}
	return o.facade.SSHClientConfig()
}

// Expiry returns the credential expiry.
func (o *UnstableClientCredentialOutput) Expiry() (time.Time, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.facade == nil {
		return time.Time{}, false
	}
	return o.facade.Expiry()
}

// Facade returns the underlying facade
func (o *UnstableClientCredentialOutput) Facade() (*identity.Facade, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.facade == nil {
		return nil, trace.BadParameter("credentials not yet ready")
	}
	return o.facade, nil
}

// SetOrUpdateFacade sets up the underlying facade or updates it if it has
// already been created.
func (o *UnstableClientCredentialOutput) SetOrUpdateFacade(id *identity.Identity) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.facade == nil {
		if o.ready != nil {
			close(o.ready)
		}
		o.facade = identity.NewFacade(false, false, id)
		return
	}
	o.facade.Set(id)
}

// CheckAndSetDefaults implements the Destination interface and does nothing in
// this implementation.
func (o *UnstableClientCredentialOutput) CheckAndSetDefaults() error {
	return nil
}

// MarshalYAML enables the yaml package to correctly marshal the Destination
// as YAML including the type header.
func (o *UnstableClientCredentialOutput) MarshalYAML() (interface{}, error) {
	type raw UnstableClientCredentialOutput
	return withTypeHeader((*raw)(o), UnstableClientCredentialOutputType)
}

// Type returns a human readable description of this output.
func (o *UnstableClientCredentialOutput) Type() string {
	return UnstableClientCredentialOutputType
}

func (o *UnstableClientCredentialOutput) GetCredentialLifetime() CredentialLifetime {
	return CredentialLifetime{}
}
