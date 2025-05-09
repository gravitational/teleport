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

package azuredevops

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
)

// providerTimeout is the maximum time allowed to fetch provider metadata before
// giving up.
const providerTimeout = 15 * time.Second

// audience is the static value that Azure DevOps uses for the `aud` claim in
// issued ID Tokens. Unfortunately, this cannot be changed.
const audience = "api://AzureADTokenExchange"

func issuerURL(organizationID string) string {
	issuerURL := url.URL{
		Scheme: "https",
		Host:   "vstoken.dev.azure.com",
		Path:   fmt.Sprintf("/%s", organizationID),
	}
	return issuerURL.String()
}

// IDTokenValidator validates an Azure Devops issued ID Token.
type IDTokenValidator struct {
	// clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	clock clockwork.Clock
}

// NewIDTokenValidator returns an initialized IDTokenValidator
func NewIDTokenValidator(clock clockwork.Clock) *IDTokenValidator {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	return &IDTokenValidator{
		clock: clock,
	}
}

// Validate validates an Azure Devops issued ID token.
func (id *IDTokenValidator) Validate(
	ctx context.Context, organizationID, token string,
) (*IDTokenClaims, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()

	issuer := issuerURL(organizationID)
	dc, err := client.Discover(timeoutCtx, issuer, http.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err, "discovering oidc document")
	}

	ks := rp.NewRemoteKeySet(http.DefaultClient, dc.JwksURI)
	verifier := rp.NewIDTokenVerifier(issuer, audience, ks)
	// TODO: Figure out injection of clock for testing

	claims, err := rp.VerifyIDToken[*IDTokenClaims](timeoutCtx, token, verifier)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	parsed, err := parseSubClaim(claims.Sub)
	if err != nil {
		return nil, trace.Wrap(err, "parsing sub claim")
	}
	claims.OrganizationName = parsed.organizationName
	claims.ProjectName = parsed.projectName
	claims.PipelineName = parsed.pipelineName

	return claims, nil
}

type parsedSubClaim struct {
	organizationName string
	projectName      string
	pipelineName     string
}

func parseSubClaim(sub string) (parsedSubClaim, error) {
	parsed, err := url.Parse(sub)
	if err != nil {
		return parsedSubClaim{}, trace.Wrap(err, "parsing as url")
	}

	// Special p:// scheme indicates this is a Pipeline ID token rather than
	// a service connection ID token (which starts sc://).
	if parsed.Scheme != "p" {
		return parsedSubClaim{}, trace.BadParameter(
			"id token is not of pipeline kind (sub: %q)", sub,
		)
	}

	out := parsedSubClaim{}
	out.organizationName = parsed.Host
	// Now we need to handle the path, which is something like
	// /project-name/pipeline-name
	path, _ := strings.CutPrefix(parsed.Path, "/")
	split := strings.Split(path, "/")
	if len(split) != 2 {
		return parsedSubClaim{}, trace.BadParameter(
			"subject malformed, expected 2 path elements (sub: %q)", sub,
		)
	}
	out.projectName = split[0]
	out.pipelineName = split[1]

	return out, nil
}
