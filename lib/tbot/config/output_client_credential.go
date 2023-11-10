/*
Copyright 2023 Gravitational, Inc.

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

package config

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"golang.org/x/crypto/ssh"
	"sync"
)

// Assert that this UnstableClientCredentialOutput can be used as client
// credential.
var _ client.Credentials = new(UnstableClientCredentialOutput)

const UnstableClientCredentialOutputType = "unstable_client_credential"

type UnstableClientCredentialOutput struct {
	mu     sync.Mutex
	facade *identity.Facade
	ready  chan struct{}
}

func (o *UnstableClientCredentialOutput) Ready() <-chan struct{} {

	return nil // TODO: Implement
}

func (o *UnstableClientCredentialOutput) Dialer(c client.Config) (client.ContextDialer, error) {
	return o.facade.Dialer(c)
}

func (o *UnstableClientCredentialOutput) TLSConfig() (*tls.Config, error) {
	return o.facade.TLSConfig()
}

func (o *UnstableClientCredentialOutput) SSHClientConfig() (*ssh.ClientConfig, error) {
	return o.facade.SSHClientConfig()
}

func (o *UnstableClientCredentialOutput) Render(_ context.Context, _ provider, ident *identity.Identity) error {
	// We're hijacking the Render method to receive a new identity in each
	// renewal round.
	o.facade.Set(ident)
	return nil
}

func (o *UnstableClientCredentialOutput) Init(ctx context.Context) error {
	// Nothing to do.
	return nil
}

func (o *UnstableClientCredentialOutput) GetDestination() bot.Destination {
	return &DestinationNop{}
}

func (o *UnstableClientCredentialOutput) GetRoles() []string {
	return []string{}
}

func (o *UnstableClientCredentialOutput) CheckAndSetDefaults() error {
	// Nothing to check!
	return nil
}

func (o *UnstableClientCredentialOutput) Describe() []FileDescription {
	// Produces no files.
	return []FileDescription{}
}

func (o *UnstableClientCredentialOutput) MarshalYAML() (interface{}, error) {
	type raw UnstableClientCredentialOutput
	return withTypeHeader((*raw)(o), UnstableClientCredentialOutputType)
}

func (o *UnstableClientCredentialOutput) String() string {
	return fmt.Sprintf("%s", UnstableClientCredentialOutputType)
}
