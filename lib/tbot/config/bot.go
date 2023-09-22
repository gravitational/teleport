/*
Copyright 2022 Gravitational, Inc.

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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/tbot/identity"
)

// Bot is an interface covering various public tbot.Bot methods to circumvent
// import cycle issues.
type Bot interface {
	// AuthPing pings the auth server and returns the (possibly cached) response.
	AuthPing(ctx context.Context) (*proto.PingResponse, error)

	// ProxyPing returns a (possibly cached) ping response from the Teleport proxy.
	// Note that it relies on the auth server being configured with a sane proxy
	// public address.
	ProxyPing(ctx context.Context) (*webclient.PingResponse, error)

	// GetCertAuthorities returns the possibly cached CAs of the given type and
	// requests them from the server if unavailable.
	GetCertAuthorities(ctx context.Context, caType types.CertAuthType) ([]types.CertAuthority, error)

	// AuthenticatedUserClientFromIdentity returns a client backed by a specific
	// identity.
	AuthenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity) (auth.ClientI, error)

	// Config returns the current bot config
	Config() *BotConfig
}
