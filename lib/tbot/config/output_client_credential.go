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
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const ClientCredentialOutputType = "client_credential"

type ClientCredentialOutput struct {
}

func (o *ClientCredentialOutput) Wait(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (o *ClientCredentialOutput) Dialer(cfg client.Config) (client.ContextDialer, error) {
	//TODO implement me
	panic("implement me")
}

func (o *ClientCredentialOutput) TLSConfig() (*tls.Config, error) {
	//TODO implement me
	panic("implement me")
}

func (o *ClientCredentialOutput) SSHClientConfig() (*ssh.ClientConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (o *ClientCredentialOutput) Render(ctx context.Context, p provider, ident *identity.Identity) error {
	dest := o.GetDestination()
	if err := identity.SaveIdentity(ctx, ident, dest, identity.DestinationKinds()...); err != nil {
		return trace.Wrap(err, "persisting identity")
	}

	return nil
}

func (o *ClientCredentialOutput) Init(ctx context.Context) error {
	return nil
}

func (o *ClientCredentialOutput) GetDestination() bot.Destination {
	return &DestinationNop{}
}

func (o *ClientCredentialOutput) GetRoles() []string {
	return []string{}
}

func (o *ClientCredentialOutput) CheckAndSetDefaults() error {
	return nil
}

func (o *ClientCredentialOutput) Describe() []FileDescription {
	return []FileDescription{}
}

func (o ClientCredentialOutput) MarshalYAML() (interface{}, error) {
	type raw ClientCredentialOutput
	return withTypeHeader(raw(o), ClientCredentialOutputType)
}

func (o *ClientCredentialOutput) String() string {
	return fmt.Sprintf("%s", ClientCredentialOutputType)
}
