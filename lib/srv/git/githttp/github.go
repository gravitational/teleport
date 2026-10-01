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

package githttp

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	appcommon "github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
)

func (h *Handler) handleGitHub(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, gitServer types.Server) error {
	switch r.Host {
	case types.PublicGitHubHost:
		return h.handleGitHubGit(ctx, logger, w, r, identity, gitServer)
	case types.PublicGitHubAPIHost:
		return h.handleGitHubAPI(ctx, logger, w, r, identity, gitServer)
	default:
		return trace.AccessDenied("unsupported GitHub host %q", r.Host)
	}
}

// handleGitHubGit handles Git smart HTTP requests to github.com.
func (h *Handler) handleGitHubGit(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, gitServer types.Server) error {
	// Do our best to verify the org matches.
	if err := checkGitHubRepoOwner(r.URL.Path, gitServer.GetGitHub().Organization); err != nil {
		return trace.Wrap(err)
	}

	return h.forwardToGitHub(ctx, logger, w, r, func(out *http.Request, accessToken string) {
		// Git HTTPS uses HTTP Basic with "x-access-token" as username:
		// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
		out.SetBasicAuth("x-access-token", accessToken)
	})
}

// handleGitHubAPI handles REST and GraphQL requests to api.github.com.
func (h *Handler) handleGitHubAPI(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, r *http.Request, identity *tlsca.Identity, gitServer types.Server) error {
	// Note that there is no easy and reliable way to ensure organization by
	// checking the request. Therefore, we solely rely on the token being scoped
	// to the organization's GitHub App installation.
	return h.forwardToGitHub(ctx, logger, w, r, func(out *http.Request, accessToken string) {
		// REST API uses Bearer token:
		// https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api
		out.Header.Set("Authorization", "Bearer "+accessToken)
	})
}

func checkGitHubRepoOwner(urlPath, org string) error {
	owner, _, _ := strings.Cut(strings.TrimPrefix(path.Clean(urlPath), "/"), "/")
	if !strings.EqualFold(owner, org) {
		return trace.AccessDenied("repository owner %q does not match organization %q", owner, org)
	}
	return nil
}

// forwardToGitHub decrypts the upstream token and forwards the request to
// GitHub. setAuth sets the upstream credentials on the outgoing request.
func (h *Handler) forwardToGitHub(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, r *http.Request, setAuth func(out *http.Request, accessToken string)) error {
	encryptedToken := r.Header.Get(teleport.GitTokenHeader)
	if encryptedToken == "" {
		return trace.AccessDenied("missing %s header", teleport.GitTokenHeader)
	}
	userCert, err := authz.UserCertificateFromContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	accessToken, err := h.cfg.UpstreamAuth.DecryptToken(ctx, userCert.Raw, encryptedToken)
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(ctx, "Forwarding request to GitHub",
		"method", r.Method,
		"host", r.Host,
		"path", r.URL.Path,
	)

	// TODO(greedy52) add session recording and audit events.
	fwd, err := reverseproxy.New(
		reverseproxy.WithRoundTripper(h.cfg.Transport),
		reverseproxy.WithLogger(logger),
		reverseproxy.WithRewriter(newGitHubRequestRewriter(accessToken, setAuth)),
		reverseproxy.WithErrorHandler(func(w http.ResponseWriter, req *http.Request, err error) {
			logger.ErrorContext(req.Context(), "GitHub upstream error", "error", err)
			trace.WriteError(w, trace.ConnectionProblem(err, "upstream error"))
		}),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	fwd.ServeHTTP(w, r)
	return nil
}

func newGitHubRequestRewriter(accessToken string, setAuth func(out *http.Request, accessToken string)) reverseproxy.Rewriter {
	defaultHeaderRewriter := reverseproxy.NewHeaderRewriter()
	defaultHeaderRewriter.TrustForwardHeader = false

	return appcommon.NewHeaderRewriter(
		&gitHubRequestRewriter{
			accessToken: accessToken,
			setAuth:     setAuth,
		},
		defaultHeaderRewriter,
	)
}

type gitHubRequestRewriter struct {
	accessToken string
	setAuth     func(out *http.Request, accessToken string)
}

func (rw *gitHubRequestRewriter) Rewrite(req *httputil.ProxyRequest) {
	out := req.Out
	out.URL.Scheme = "https"
	out.URL.Host = req.In.Host
	out.URL.User = nil

	out.Header.Del("Authorization")
	out.Header.Del("Cookie")
	out.Header.Del(teleport.GitTokenHeader)

	rw.setAuth(out, rw.accessToken)
}
