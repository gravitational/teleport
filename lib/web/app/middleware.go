/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package app

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/lib/utils"
)

// withRouterAuth authenticates requests then hands the request to a
// httprouter.Handler handler.
func (h *Handler) withRouterAuth(handler routerAuthFunc) httprouter.Handle {
	return makeRouterHandler(func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
		session, err := h.authenticate(r.Context(), r)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := handler(w, r, p, session); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

// withAuth authenticates requests then hands the request to a http.HandlerFunc
// handler.
func (h *Handler) withAuth(handler handlerAuthFunc) http.HandlerFunc {
	return makeHandler(func(w http.ResponseWriter, r *http.Request) error {
		// If the caller fails to authenticate, redirect the caller to Teleport.
		session, err := h.authenticate(r.Context(), r)
		if err != nil {
			if redirectErr := h.redirectToLauncher(w, r, launcherURLParams{}); redirectErr == nil {
				return nil
			}
			return trace.Wrap(err)
		}
		if err := handler(w, r, session); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

// redirectToLauncher redirects to the proxy web's app launcher if the public
// address of the proxy is set.
func (h *Handler) redirectToLauncher(w http.ResponseWriter, r *http.Request, p launcherURLParams) error {
	if p.stateToken == "" && !HasSessionCookie(r) {
		// Reaching this block means the application was accessed through the CLI (eg: tsh app login)
		// and there was a forwarding error and we could not renew the app web session.
		// Since we can't redirect the user to the app launcher from the CLI,
		// we just return an error instead.
		return trace.BadParameter("redirecting to launcher when using client certificate, is not allowed")
	}

	if h.c.WebPublicAddr == "" {
		// The error below tends to be swallowed by the Web UI, so log a warning for
		// admins as well.
		h.log.Error("" +
			"Application Service requires public_addr to be set in the Teleport Proxy Service configuration. " +
			"Please contact your Teleport cluster administrator or refer to " +
			"https://goteleport.com/docs/application-access/guides/connecting-apps/#start-authproxy-service.")
		return trace.BadParameter("public address of the proxy is not set")
	}
	addr, err := utils.ParseAddr(r.Host)
	if err != nil {
		return trace.Wrap(err)
	}

	urlString := makeAppRedirectURL(r, h.c.WebPublicAddr, addr.Host(), p)
	http.Redirect(w, r, urlString, http.StatusFound)
	return nil
}

// makeRouterHandler creates a httprouter.Handle.
func makeRouterHandler(handler routerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := handler(w, r, p); err != nil {
			writeError(w, err)
			return
		}
	}
}

// makeHandler creates a http.HandlerFunc.
func makeHandler(handler handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			writeError(w, err)
			return
		}
	}
}

// writeError gets the HTTP status code from trace and writes the error to the
// response writer.
func writeError(w http.ResponseWriter, err error) {
	code := trace.ErrorToCode(err)
	http.Error(w, http.StatusText(code), code)
}

type routerFunc func(http.ResponseWriter, *http.Request, httprouter.Params) error
type routerAuthFunc func(http.ResponseWriter, *http.Request, httprouter.Params, *session) error

type handlerAuthFunc func(http.ResponseWriter, *http.Request, *session) error
type handlerFunc func(http.ResponseWriter, *http.Request) error

type launcherURLParams struct {
	// clusterName is the cluster within which this application is running.
	clusterName string
	// publicAddr is the public address of this application.
	publicAddr string
	// arn is the AWS role name, defined only when accessing AWS management console.
	arn string
	// stateToken if defined means initiating an app access auth exchange.
	stateToken string
	// path is the application URL path.
	// It is only defined if an application was accessed without the web launcher
	// (e.g: clicking on a bookmarked URL).
	// This field is used to preserve the original requested path through
	// the app access authentication redirections.
	path string
}
