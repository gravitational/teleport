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

	h.forward(w, r, session)
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

		// Extract the session cookie from the *http.Request.
		cookie, err := decodeCookie(req.CookieValue)
		if err != nil {
			return trace.Wrap(err)
		}

		// Validate that the session exists.
		_, err = h.sessions.get(cookie)
		if err != nil {
			return trace.Wrap(err)
		}

		// Encode cookie and set the "Set-Cookie" header on the response.
		cookieValue, err := encodeCookie(cookie)
		if err != nil {
			return trace.Wrap(err)
		}
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
	return &session{}, nil

	//// Extract the session cookie from the *http.Request.
	//cookie, err := decodeCookie(r)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}

	//session, err := h.sessions.get(cookie)
	//if err != nil {
	//	return nil, trace.Wrap(err)
	//}

	////// A session exists in the backend. It should contain identity information.
	////roleset, err := services.FetchRoles(roles, clt, traits)
	////if err != nil {
	////	return nil, trace.Wrap(err)
	////}

	////err = s.checker.CheckAccessToApp(s.app, r)
	////if err != nil {
	////	http.Error(w, fmt.Sprintf("access to app %v denied", s.app.GetName()), 401)
	////	return
	////}
	////fmt.Printf("--> checker.CheckAccessToApp: %v.\n", err)

	//return session, nil
}

func (h *Handler) forward(w http.ResponseWriter, r *http.Request, session *session) {
	fmt.Fprintf(w, "hello, world")
	/*
		// Get a net.Conn over the reverse tunnel connection.
		conn, err := s.clusterClient.Dial(reversetunnel.DialParams{
			ServerID: strings.Join([]string{s.app.GetHostUUID(), s.clusterClient.GetName()}, "."),
			ConnType: services.AppTunnel,
		})
		if err != nil {
			// TODO: This should say something else, like application not available to
			// the user and log the actual reason the application was down for the admin.
			// connection rejected: dial tcp 127.0.0.1:8081: connect: connection refused.
			fmt.Printf("--> Dial: %v.\n", err)
			http.Error(w, "internal service error", 500)
			return
		}

		signedToken, err := s.jwtKey.Sign(&jwt.SignParams{
			Email: s.identity.Username,
		})
		if err != nil {
			fmt.Printf("--> get signed token: %v.\n", err)
			http.Error(w, "internal service error", 500)
			return
		}

		// Forward the request over the net.Conn to the host running the application within the cluster.
		roundTripper := forward.RoundTripper(newCustomTransport(conn))
		fwd, _ := forward.New(roundTripper, forward.Rewriter(&rewriter{signedToken: signedToken}))

		//nu, _ := url.Parse(r.URL.String())
		//nu.Scheme = "https"
		//nu.Host = "rusty-gitlab.gravitational.io"
		r.URL = testutils.ParseURI("http://localhost:8081")

		// let us forward this request to another server
		//r.URL = testutils.ParseURI("https://rusty-gitlab.gravitational.io")
		//r.URL = testutils.ParseURI("localhost:8080")

		fwd.ServeHTTP(w, r)
	*/
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

/*
	// Exchange nonce for a session.
	nonce := r.URL.Query().Get("nonce")
	if nonce != "" {
		session, err := h.appsHandler.AuthClient.ExchangeNonce(r.Context(), nonce)
		if err != nil {
			fmt.Printf("--> ExchangeNonce failed: %.v\n", err)
			http.Error(w, "access denied", 401)
			return
		}

		fmt.Printf("--> ExchangeNonce: session.GetName(): %v.\n", session.GetName())

		// If the user was able to successfully exchange an existing web session
		// token for a app session token, create a cookie from it and set it on the
		// response.
		cookie, err := apps.CookieFromSession(session)
		if err != nil {
			// TODO: What should happen here, show an error to the user?
			http.Error(w, "apps.CookieFromSession failed", 401)
			return
		}
		http.SetCookie(w, cookie)
		http.Redirect(w, r, "https://dumper.proxy.example.com:3080", 302)
		return
	}

	// TODO: This client needs to be the cluster within which the application
	// is running.
	clusterClient, err := h.handler.cfg.Proxy.GetSite("example.com")
	if err != nil {
		http.Error(w, "access denied", 401)
		return
	}
	r := r.WithContext(context.WithValue(r.Context(), "clusterClient", clusterClient))

	r = r.WithContext(context.WithValue(r.Context(), "app", app))
	h.appsHandler.ServeHTTP(w, r)
	return

	//// Verify with @alex-kovoy that it's okay that bearer token is false. This
	//// appears to make sense because the bearer token is injected client side
	//// and that's not possible for AAP.
	//ctx, err := h.handler.AuthenticateRequest(w, r, false)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}

	//// Attach certificates (x509 and SSH) to *http.Request.
	//_, cert, err := ctx.GetCertificates()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r := r.WithContext(context.WithValue(r.Context(), "identity", identity))

	//// Attach services.RoleSet to *http.Request.
	//checker, err := ctx.GetRoleSet()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r = r.WithContext(context.WithValue(r.Context(), "checker", checker))

	//// Attach services.App requested to the *http.Request.
	//r = r.WithContext(context.WithValue(r.Context(), "app", app))

	//// Attach the cluster API to the request as well.
	//// TODO: Attach trusted cluster site if trusted cluster requested.
	//clusterName, err := h.appsHandler.AuthClient.GetDomainName()
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//}
	//clusterClient, err := h.handler.cfg.Proxy.GetSite(clusterName)
	//if err != nil {
	//	http.Error(w, "access denied", 401)
	//	return
	//}
	//r = r.WithContext(context.WithValue(r.Context(), "clusterName", clusterName))
	//r = r.WithContext(context.WithValue(r.Context(), "clusterClient", clusterClient))

	//// Pass the request along to the apps handler.
	//h.appsHandler.ServeHTTP(w, r)
	//return
*/
