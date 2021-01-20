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
	"fmt"
	"net/http"
	"net/url"

	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/util/validation"
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
			// If this cluster has the public address of the proxy set, redirect to the
			// login page to redirect to the requested application. If not, the best
			// that can be done is to show "403 Forbidden" because Teleport does not know
			// the public address of the Web UI which is needed to launch the application.
			if h.c.WebPublicAddr != "" {
				addr, err := utils.ParseAddr(r.Host)
				if err != nil {
					return trace.Wrap(err)
				}
				// While the FQDN will be validated when creating an application session,
				// make sure we don't send something that doesn't at least look like a
				// FQDN to the frontend application.
				if errs := validation.IsDNS1123Subdomain(addr.Host()); len(errs) > 0 {
					var aerrs []error
					for _, err := range errs {
						aerrs = append(aerrs, trace.BadParameter(err))
					}
					return trace.NewAggregate(aerrs...)
				}
				u := url.URL{
					Scheme: "https",
					Host:   h.c.WebPublicAddr,
					Path:   fmt.Sprintf("/web/launch/%v", addr.Host()),
				}
				http.Redirect(w, r, u.String(), http.StatusFound)
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
