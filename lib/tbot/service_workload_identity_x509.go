// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package tbot

import (
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/spiffe"
)

// WorkloadIdentityX509Service
type WorkloadIdentityX509Service struct {
	botAuthClient  *authclient.Client
	botCfg         *config.BotConfig
	cfg            *config.WorkloadIdentityOutput
	getBotIdentity getBotIdentityFn
	log            *slog.Logger
	resolver       reversetunnelclient.Resolver
	// trustBundleCache is the cache of trust bundles. It only needs to be
	// provided when running in daemon mode.
	trustBundleCache *spiffe.TrustBundleCache
}
