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

package web

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/httplib"
)

type githubIntegrationCallbackResponse struct {
	RedirectURL string `json:"redirectURL"`
}

func (h *Handler) githubIntegrationCallback(w http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *SessionContext) (any, error) {
	var req struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Code == "" || req.State == "" {
		return nil, trace.BadParameter("missing code or state")
	}

	authRequest, err := h.cfg.ProxyClient.GetGithubAuthRequest(r.Context(), req.State)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Extra checks on authenticated user.
	switch authRequest.AuthenticatedUser {
	case sctx.GetUser():
	case "":
		return h.githubIntegrationErrorRedirect(
			r.Context(),
			authRequest.ClientRedirectURL,
			trace.BadParameter("auth request is not for an authenticated user"),
		)
	default:
		h.logger.WarnContext(r.Context(), "GitHub integration callback user mismatch",
			"session_user", sctx.GetUser(),
			"auth_request_user", authRequest.AuthenticatedUser,
		)
		return h.githubIntegrationErrorRedirect(
			r.Context(),
			authRequest.ClientRedirectURL,
			trace.AccessDenied("session user does not match the user who initiated the OAuth flow"),
		)
	}

	// Resume rest of the regular oauth flow.
	q := make(url.Values)
	q.Set("code", req.Code)
	q.Set("state", req.State)

	response, err := h.cfg.ProxyClient.ValidateGithubAuthCallback(r.Context(), q)
	if err != nil {
		return h.githubIntegrationErrorRedirect(r.Context(), authRequest.ClientRedirectURL, err)
	}

	redirectURL, err := ConstructSSHResponse(AuthParams{
		ClientRedirectURL: response.Req.ClientRedirectURL,
		Username:          response.Username,
		Identity:          response.Identity,
		Session:           response.Session,
		Cert:              response.Cert,
		TLSCert:           response.TLSCert,
		HostSigners:       response.HostSigners,
		FIPS:              h.cfg.FIPS,
		ClientOptions:     response.ClientOptions,
	})
	if err != nil {
		return h.githubIntegrationErrorRedirect(r.Context(), response.Req.ClientRedirectURL, err)
	}

	return &githubIntegrationCallbackResponse{
		RedirectURL: redirectURL.String(),
	}, nil
}

// githubIntegrationErrorRedirect attempts to redirect back to the client with
// the error. This improves the UX by terminating the failed OAuth flow
// immediately, rather than hoping for a timeout.
func (h *Handler) githubIntegrationErrorRedirect(ctx context.Context, clientRedirectURL string, err error) (any, error) {
	if clientRedirectURL != "" {
		if redURL, errEnc := RedirectURLWithError(clientRedirectURL, err); errEnc == nil {
			h.logger.ErrorContext(ctx, "Github integration callback error", "redirect_url", redURL, "error", err)
			return &githubIntegrationCallbackResponse{
				RedirectURL: redURL.String(),
			}, nil
		}
	}
	return nil, trace.Wrap(err)
}
