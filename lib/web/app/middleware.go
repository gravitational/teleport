/*
Copyright 2020 Gravitational, Inc.

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
			if redirectErr := h.redirectToLauncher(w, r); redirectErr == nil {
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
func (h *Handler) redirectToLauncher(w http.ResponseWriter, r *http.Request) error {
	// The application launcher can only generate browser sessions (based on
	// Cookies). Given this, we should only redirect to it when this format is
	// already in use.
	if !HasSession(r) {
		return trace.BadParameter("redirecting to launcher when using client certificate is not valid")
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

	urlString := makeAppRedirectURL(r, h.c.WebPublicAddr, addr.Host())
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
