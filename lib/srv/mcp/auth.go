/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package mcp

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/services"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

// maxTokenDuration defines how long an egress JWT or ID token should last. Most
// providers use one hour as default. However, since we divide streamable-HTTP
// requests in chunks which are 5 minutes, our token doesn't have to be that
// long. For SSE servers, a refresh will happen after the token expires upon new
// requests.
const maxTokenDuration = time.Minute * 10

// sessionAuth handles generating auth tokens for an MCP session.
type sessionAuth struct {
	*SessionCtx
	authClient appcommon.AppTokenGenerator
	clock      clockwork.Clock

	mu         sync.Mutex
	jwt        string
	traits     wrappers.Traits
	lastUpdate time.Time
}

func (a *sessionAuth) generateJWTAndTraits(ctx context.Context) (string, wrappers.Traits, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.clock.Now().Before(a.lastUpdate.Add(maxTokenDuration)) {
		return a.jwt, a.traits, nil
	}

	// Note that token validation on server side usually has some leeway like a
	// minute, so we don't have to worry about adding that to "expires".
	now := a.clock.Now()
	expires := a.Identity.Expires
	if maxExpires := now.Add(maxTokenDuration); maxExpires.Before(expires) {
		expires = maxExpires
	}

	jwt, traitsForRewriteHeaders, err := appcommon.GenerateJWTAndTraits(ctx, &a.Identity, a.App, a.authClient, expires)
	if err != nil {
		return "", nil, trace.Wrap(err)
	}

	if a.rewriteAuthDetails.hasIDTokenTrait {
		idToken, err := generateIDToken(ctx, &a.Identity, a.App, a.authClient, expires)
		if err != nil {
			return "", nil, trace.Wrap(err)
		}
		traitsForRewriteHeaders[constants.TraitIDToken] = []string{idToken}
	}

	a.jwt = jwt
	a.traits = traitsForRewriteHeaders
	a.lastUpdate = now
	return jwt, traitsForRewriteHeaders, nil
}

type rewriteAuthDetails struct {
	rewriteAuthHeader bool
	hasIDTokenTrait   bool
	hasJWTTrait       bool
}

// rewriteTraitsTest are fake traits used for testing if {{internal.jwt}} and
// {{internal.id_token}} are defined in app rewrite. The number of elements in
// the slice can be used to quickly tell which traits have been applied.
var rewriteTraitsTest = wrappers.Traits{
	constants.TraitJWT:     {"j", "w", "t"},
	constants.TraitIDToken: {"i", "d"},
}

func newRewriteAuthDetails(rewrite *types.Rewrite) rewriteAuthDetails {
	if rewrite == nil {
		return rewriteAuthDetails{}
	}

	var r rewriteAuthDetails
	for _, header := range rewrite.Headers {
		if strings.EqualFold(header.Name, "Authorization") {
			r.rewriteAuthHeader = true
		}

		interpolated, _ := services.ApplyValueTraits(header.Value, rewriteTraitsTest)
		switch len(interpolated) {
		case 3:
			r.hasJWTTrait = true
		case 2:
			r.hasIDTokenTrait = true
		}
	}
	return r
}

func generateIDToken(ctx context.Context, identity *tlsca.Identity, app types.Application, auth AuthClient, expires time.Time) (string, error) {
	roles, traits := appcommon.RolesAndTraitsForAppToken(identity, app)

	// Use types.OIDCIdPCA to generate the token.
	idToken, err := auth.GenerateAppToken(ctx, types.GenerateAppTokenRequest{
		Username:      identity.Username,
		Roles:         roles,
		Traits:        traits,
		URI:           app.GetURI(),
		Expires:       expires,
		AuthorityType: types.OIDCIdPCA,
	})
	return idToken, trace.Wrap(err)
}
