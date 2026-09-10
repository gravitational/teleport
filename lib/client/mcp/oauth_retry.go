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
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils"
)

// NewOAuthRetryRoundTripper returns a round tripper that, when the server
// answers with a Bearer invalid_token challenge, asks refreshAuthHeader for a
// new Authorization header and replays the request once. A body that cannot be
// re-read is buffered before the first attempt, bounded by
// teleport.MaxHTTPRequestSize.
func NewOAuthRetryRoundTripper(base http.RoundTripper, refreshAuthHeader func(ctx context.Context, rejectedHeader string) (string, error)) http.RoundTripper {
	return &oauthRetryRoundTripper{base: base, refreshAuthHeader: refreshAuthHeader}
}

type oauthRetryRoundTripper struct {
	base              http.RoundTripper
	refreshAuthHeader func(context.Context, string) (string, error)
}

func (t *oauthRetryRoundTripper) CloseIdleConnections() {
	type closeIdler interface {
		CloseIdleConnections()
	}
	if base, ok := t.base.(closeIdler); ok {
		base.CloseIdleConnections()
	}
}

func (t *oauthRetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req, err := withReplayableBody(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := t.base.RoundTrip(req)
	if err != nil || !isOAuthInvalidTokenResponse(resp) {
		return resp, err
	}

	retry, err := cloneRequestForOAuthRetry(req)
	if err != nil {
		if resp.Body != nil {
			resp.Body.Close()
		}
		return nil, trace.Wrap(err)
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

// isOAuthInvalidTokenResponse reports whether a 401 carries a Bearer challenge
// with error="invalid_token", the only error code a token refresh can fix:
// https://www.rfc-editor.org/rfc/rfc6750.html#section-3.1
func isOAuthInvalidTokenResponse(resp *http.Response) bool {
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		return false
	}
	for _, challenge := range resp.Header.Values("WWW-Authenticate") {
		if strings.Contains(challenge, "invalid_token") {
			return true
		}
	}
	return false
}

// withReplayableBody buffers a body that has no GetBody so the request can be
// replayed. Requests built by http.NewRequest from an in-memory reader already
// carry GetBody; requests proxied from an http.Server do not.
func withReplayableBody(req *http.Request) (*http.Request, error) {
	if req.Body == nil || req.Body == http.NoBody || req.GetBody != nil {
		return req, nil
	}
	body, err := utils.ReadAtMost(req.Body, teleport.MaxHTTPRequestSize)
	_ = req.Body.Close()
	if err != nil {
		return nil, trace.Wrap(err, "buffering request body for OAuth retry")
	}
	req = req.Clone(req.Context())
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return req, nil
}

func cloneRequestForOAuthRetry(req *http.Request) (*http.Request, error) {
	retry := req.Clone(req.Context())
	if req.GetBody == nil {
		return retry, nil
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, trace.Wrap(err, "recreating request body for OAuth retry")
	}
	retry.Body = body
	return retry, nil
}
