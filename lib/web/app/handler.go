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

// Package app
package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

type HandlerConfig struct {
	AuthClient  auth.ClientI
	ProxyClient reversetunnel.Server
}

func (c *HandlerConfig) Check() error {
	if c.AuthClient == nil {
		return trace.BadParameter("auth client missing")
	}
	if c.ProxyClient == nil {
		return trace.BadParameter("proxy client missing")
	}

	return nil
}

type Handler struct {
	c   HandlerConfig
	log *logrus.Entry

	sessions *sessionCache
}

func NewHandler(config HandlerConfig) (*Handler, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	sessionCache, err := newSessionCache()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Handler{
		c: config,
		log: logrus.WithFields(logrus.Fields{
			trace.Component: teleport.ComponentAppProxy,
		}),
		sessions: sessionCache,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// If the target is an application but it hits the special "x-teleport-auth"
	// endpoint, then perform redirect authentication logic.
	if r.URL.Path == "/x-teleport-auth" {
		if err := h.handleFragment(w, r); err != nil {
			h.log.Warnf("Fragment authentication failed: %v.", err)
			http.Error(w, "internal service error", 500)
			return
		}
	}

	// Authenticate request by looking for an existing session. If a session
	// does not exist, redirect the caller to the login screen.
	session, err := h.authenticate(r)
	if err != nil {
		h.log.Warnf("Authentication failed: %v.", err)
		http.Error(w, "internal service error", 500)
		return
	}

	err = h.forward(w, r, session)
	if err != nil {
		h.log.Warnf("Authentication failed: %v.", err)
		http.Error(w, "internal service error", 500)
		return
	}
}

// TODO(russjones): Add support for trusted clusters here? Or should that
// only happen in the session cookie?
func (h *Handler) IsApp(r *http.Request) (services.Server, error) {
	appName, err := extractAppName(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := h.c.AuthClient.GetApp(r.Context(), defaults.Namespace, appName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

type fragmentRequest struct {
	CookieValue string `json:"cookie_value"`
}

func (h *Handler) handleFragment(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodGet:
		fmt.Fprintf(w, js)
	case http.MethodPost:
		var req fragmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return trace.Wrap(err)
		}

		// Validate that the session exists.
		_, err = h.sessions.get(r.Context(), req.CookieValue)
		if err != nil {
			return trace.Wrap(err)
		}

		// TODO(russjones): Add additional cookie values here.
		// Set the "Set-Cookie" header on the response.
		http.SetCookie(w, &http.Cookie{
			Name:  cookieName,
			Value: cookieValue,
		})
	default:
		return trace.BadParameter("unsupported method: %q", r.Method)
	}
	return nil
}

func (h *Handler) authenticate(r *http.Request) (*session, error) {
	// Extract the session cookie from the *http.Request.
	cookieValue, err := extractCookie(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check the cache for an authenticated session.
	session, err := h.sessions.get(cookieValue)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// TODO(russjones): In this function, if forwarding request fails, return
// an error to the user, and delete the *session and force it to be recreated
// to allow the user of new connection through the tunnel.
func (h *Handler) forward(w http.ResponseWriter, r *http.Request, s *session) error {
	// Check access to the target application before forwarding. This allows an
	// admin to change roles assigned an user/application at runtime and deny
	// access to the application.
	//
	// This code path should be profiled if it ever becomes a bottleneck.
	err := s.checker.CheckAccessToApp(s.app)
	if err != nil {
		return trace.Wrap(err)
	}

	r.URL = testutils.ParseURI("http://localhost:8081")
	s.fwd.ServeHTTP(w, r)
	return nil
}

func extractAppName(r *http.Request) (string, error) {
	requestedHost, err := utils.Host(r.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}

	parts := strings.FieldsFunc(requestedHost, func(c rune) bool {
		return c == '.'
	})
	if len(parts) == 0 {
		return "", trace.BadParameter("invalid host header: %v", requestedHost)
	}

	return parts[0], nil
}
