/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"net/http"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
)

var oauthInvalidTokenErrorPattern = regexp.MustCompile(`(?i)(?:^|[,\t ])error[\t ]*=[\t ]*(?:"invalid_token"|invalid_token)(?:[,\t ]|$)`)

// oauthRetryRoundTripper refreshes an OAuth token rejected explicitly as
// invalid by the upstream and retries a replayable request once.
type oauthRetryRoundTripper struct {
	base              http.RoundTripper
	refreshAuthHeader func(context.Context, string) (string, error)
}

func (t *oauthRetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil || !isOAuthInvalidTokenResponse(resp) {
		return resp, err
	}

	retry, ok, err := cloneRequestForOAuthRetry(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !ok {
		return resp, nil
	}
	if resp.Body != nil {
		resp.Body.Close()
	}

	header, err := t.refreshAuthHeader(req.Context(), req.Header.Get("Authorization"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if header == "" {
		return nil, trace.BadParameter("OAuth token refresh returned an empty Authorization header")
	}
	retry.Header.Set("Authorization", header)
	return t.base.RoundTrip(retry)
}

func isOAuthInvalidTokenResponse(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		return false
	}
	for _, challenge := range resp.Header.Values("WWW-Authenticate") {
		if strings.Contains(strings.ToLower(challenge), "bearer") &&
			oauthInvalidTokenErrorPattern.MatchString(challenge) {
			return true
		}
	}
	return false
}

func cloneRequestForOAuthRetry(req *http.Request) (*http.Request, bool, error) {
	retry := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return retry, true, nil
	}
	if req.GetBody == nil {
		return nil, false, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, false, trace.Wrap(err, "recreating request body for OAuth retry")
	}
	retry.Body = body
	return retry, true, nil
}
