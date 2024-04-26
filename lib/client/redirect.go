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

package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/secret"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	// LoginSuccessRedirectURL is a redirect URL when login was successful without errors.
	LoginSuccessRedirectURL = "/web/msg/info/login_success"

	// LoginTerminalRedirectURL is a redirect URL when login requires extra
	// action in the terminal, but was otherwise successful in the browser (ex.
	// need a hardware key tap).
	LoginTerminalRedirectURL = "/web/msg/info/login_terminal"

	// LoginFailedRedirectURL is the default redirect URL when an SSO error was encountered.
	LoginFailedRedirectURL = "/web/msg/error/login"

	// LoginFailedBadCallbackRedirectURL is a redirect URL when an SSO error specific to
	// auth connector's callback was encountered.
	LoginFailedBadCallbackRedirectURL = "/web/msg/error/login/callback"

	// LoginFailedUnauthorizedRedirectURL is a redirect URL for when an SSO authenticates successfully,
	// but the user has no matching roles in Teleport.
	LoginFailedUnauthorizedRedirectURL = "/web/msg/error/login/auth"

	// LoginClose is a redirect URL that will close the tab performing the SSO
	// login. It's used when a second tab will be opened due to the first
	// failing (such as an unmet hardware key policy) and the first should be
	// ignored.
	LoginClose = "/web/msg/info/login_close"
)

// Redirector handles SSH redirect flow with the Teleport server
type Redirector struct {
	// SSHLoginSSO contains SSH login parameters
	SSHLoginSSO
	server *httptest.Server
	mux    *http.ServeMux
	// redirectURL will be set based on the response from the Teleport
	// proxy server, will contain target redirect URL
	// to launch SSO workflow
	redirectURL utils.SyncString
	// key is a secret key used to encode/decode
	// the data with the server, it is used so that other
	// programs running on the same computer can't easilly sniff
	// the data
	key secret.Key
	// shortPath is a link-shortener path presented to the user
	// it is used to open up the browser window, notice
	// that redirectURL will be set later
	shortPath string
	// responseC is a channel to receive responses
	responseC chan *auth.SSHLoginResponse
	// errorC will contain errors
	errorC chan error
	// proxyClient is HTTP client to the Teleport Proxy
	proxyClient *WebClient
	// proxyURL is a URL to the Teleport Proxy
	proxyURL *url.URL
	// context is a close context
	context context.Context
	// cancel broadcasts cancel
	cancel context.CancelFunc
	// RedirectorConfig allows customization of Redirector
	RedirectorConfig
	// callbackAddr is the alternate URL to give to the user during login,
	// if present.
	callbackAddr string
}

// RedirectorConfig allows customization of Redirector
type RedirectorConfig struct {
	// SSOLoginConsoleRequestFn allows customizing issuance of SSOLoginConsoleReq. Optional.
	SSOLoginConsoleRequestFn func(req SSOLoginConsoleReq) (*SSOLoginConsoleResponse, error)
}

// NewRedirector returns new local web server redirector
func NewRedirector(ctx context.Context, login SSHLoginSSO, config *RedirectorConfig) (*Redirector, error) {
	clt, proxyURL, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create secret key that will be sent with the request and then used the
	// decrypt the response from the server.
	key, err := secret.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var callbackAddr string
	if login.CallbackAddr != "" {
		callbackURL, err := apiutils.ParseURL(login.CallbackAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		callbackURL.Scheme = "https"
		callbackAddr = callbackURL.String()
	}

	ctxCancel, cancel := context.WithCancel(ctx)
	rd := &Redirector{
		context:      ctxCancel,
		cancel:       cancel,
		proxyClient:  clt,
		proxyURL:     proxyURL,
		SSHLoginSSO:  login,
		mux:          http.NewServeMux(),
		key:          key,
		shortPath:    "/" + uuid.New().String(),
		responseC:    make(chan *auth.SSHLoginResponse, 1),
		errorC:       make(chan error, 1),
		callbackAddr: callbackAddr,
	}

	if config != nil {
		rd.RedirectorConfig = *config
	}

	if rd.SSOLoginConsoleRequestFn == nil {
		rd.SSOLoginConsoleRequestFn = rd.issueSSOLoginConsoleRequest
	}

	// callback is a callback URL communicated to the Teleport proxy,
	// after SAML/OIDC login, the teleport will redirect user's browser
	// to this laptop-local URL
	rd.mux.Handle("/callback", rd.wrapCallback(rd.callback))
	// short path is a link-shortener style URL
	// that will redirect to the Teleport-Proxy supplied address
	rd.mux.HandleFunc(rd.shortPath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, rd.redirectURL.Value(), http.StatusFound)
	})
	return rd, nil
}

// Start launches local http server on the machine,
// initiates SSO login request sequence with the Teleport Proxy
func (rd *Redirector) Start() error {
	if rd.BindAddr != "" {
		log.Debugf("Binding to %v.", rd.BindAddr)
		listener, err := net.Listen("tcp", rd.BindAddr)
		if err != nil {
			return trace.Wrap(err, "%v: could not bind to %v, make sure the address is host:port format for ipv4 and [ipv6]:port format for ipv6, and the address is not in use", err, rd.BindAddr)
		}
		rd.server = &httptest.Server{
			Listener: listener,
			Config: &http.Server{
				Handler:           rd.mux,
				ReadTimeout:       apidefaults.DefaultIOTimeout,
				ReadHeaderTimeout: defaults.ReadHeadersTimeout,
				WriteTimeout:      apidefaults.DefaultIOTimeout,
				IdleTimeout:       apidefaults.DefaultIdleTimeout,
			},
		}
		rd.server.Start()
	} else {
		rd.server = httptest.NewServer(rd.mux)
	}
	log.Infof("Waiting for response at: %v.", rd.server.URL)

	// communicate callback redirect URL to the Teleport Proxy
	u, err := url.Parse(rd.baseURL() + "/callback")
	if err != nil {
		return trace.Wrap(err)
	}
	query := u.Query()
	query.Set("secret_key", rd.key.String())
	u.RawQuery = query.Encode()

	req := SSOLoginConsoleReq{
		RedirectURL:          u.String(),
		PublicKey:            rd.PubKey,
		CertTTL:              rd.TTL,
		ConnectorID:          rd.ConnectorID,
		Compatibility:        rd.Compatibility,
		RouteToCluster:       rd.RouteToCluster,
		KubernetesCluster:    rd.KubernetesCluster,
		AttestationStatement: rd.AttestationStatement,
	}

	response, err := rd.SSOLoginConsoleRequestFn(req)
	if err != nil {
		return trace.Wrap(err)
	}

	// notice late binding of the redirect URL here, it is referenced
	// in the callback handler, but is known only after the request
	// is sent to the Teleport Proxy, that's why
	// redirectURL is a SyncString
	rd.redirectURL.Set(response.RedirectURL)
	return nil
}

// issueSSOLoginConsoleRequest is default implementation, but may be overridden via RedirectorConfig.IssueSSOLoginConsoleRequest.
func (rd *Redirector) issueSSOLoginConsoleRequest(req SSOLoginConsoleReq) (*SSOLoginConsoleResponse, error) {
	out, err := rd.proxyClient.PostJSON(rd.context, rd.proxyClient.Endpoint("webapi", rd.Protocol, "login", "console"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var re SSOLoginConsoleResponse
	err = json.Unmarshal(out.Bytes(), &re)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &re, nil
}

// Done is called when redirector is closed
// or parent context is closed
func (rd *Redirector) Done() <-chan struct{} {
	return rd.context.Done()
}

// ClickableURL returns a short clickable redirect URL
func (rd *Redirector) ClickableURL() string {
	if rd.server == nil {
		return "<undefined - server is not started>"
	}
	return utils.ClickableURL(rd.baseURL() + rd.shortPath)
}

func (rd *Redirector) baseURL() string {
	if rd.callbackAddr != "" {
		return rd.callbackAddr
	}
	return rd.server.URL
}

// ResponseC returns a channel with response
func (rd *Redirector) ResponseC() <-chan *auth.SSHLoginResponse {
	return rd.responseC
}

// ErrorC returns a channel with error
func (rd *Redirector) ErrorC() <-chan error {
	return rd.errorC
}

// callback is used by Teleport proxy to send back credentials
// issued by Teleport proxy
func (rd *Redirector) callback(w http.ResponseWriter, r *http.Request) (*auth.SSHLoginResponse, error) {
	if r.URL.Path != "/callback" {
		return nil, trace.NotFound("path not found")
	}

	r.ParseForm()
	if r.Form.Has("err") {
		err := r.Form.Get("err")
		return nil, trace.Errorf("identity provider callback failed with error: %v", err)
	}

	// Decrypt ciphertext to get login response.
	plaintext, err := rd.key.Open([]byte(r.Form.Get("response")))
	if err != nil {
		return nil, trace.BadParameter("failed to decrypt response: in %v, err: %v", r.URL.String(), err)
	}

	var re auth.SSHLoginResponse
	err = json.Unmarshal(plaintext, &re)
	if err != nil {
		return nil, trace.BadParameter("failed to decrypt response: in %v, err: %v", r.URL.String(), err)
	}

	return &re, nil
}

// Close closes redirector and releases all resources
func (rd *Redirector) Close() error {
	rd.cancel()
	if rd.server != nil {
		rd.server.Close()
	}
	return nil
}

// wrapCallback is a helper wrapper method that wraps callback HTTP handler
// and sends a result to the channel and redirect users to error page
func (rd *Redirector) wrapCallback(fn func(http.ResponseWriter, *http.Request) (*auth.SSHLoginResponse, error)) http.Handler {
	// Generate possible redirect URLs from the proxy URL.
	clone := *rd.proxyURL
	clone.Path = LoginFailedRedirectURL
	errorURL := clone.String()
	clone.Path = LoginSuccessRedirectURL
	successURL := clone.String()
	clone.Path = LoginClose
	closeURL := clone.String()
	clone.Path = LoginTerminalRedirectURL

	connectorName := rd.ConnectorName
	if connectorName == "" {
		connectorName = rd.ConnectorID
	}
	query := clone.Query()
	query.Set("auth", connectorName)
	clone.RawQuery = query.Encode()
	terminalRedirectURL := clone.String()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Allow", "GET, OPTIONS, POST")
		// CORS protects the _response_, and our response is always just a
		// redirect to info/login_success or error/login so it's fine to share
		// with the world; we could use the proxy URL as the origin, but that
		// would break setups where the proxy public address that tsh is using
		// is not the "main" one that ends up being used for the redirect after
		// the IdP login
		w.Header().Add("Access-Control-Allow-Origin", "*")
		switch r.Method {
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
			return
		case http.MethodGet, http.MethodPost:
		}

		response, err := fn(w, r)
		if err != nil {
			if trace.IsNotFound(err) {
				http.NotFound(w, r)
				return
			}
			select {
			case rd.errorC <- err:
			case <-rd.context.Done():
			}
			redirectURL := errorURL
			// A second SSO login attempt will be initiated if a key policy requirement was not satisfied.
			if requiredPolicy, err := keys.ParsePrivateKeyPolicyError(err); err == nil && rd.ProxySupportsKeyPolicyMessage {
				switch requiredPolicy {
				case keys.PrivateKeyPolicyHardwareKey, keys.PrivateKeyPolicyHardwareKeyTouch:
					// No user interaction required.
					redirectURL = closeURL
				case keys.PrivateKeyPolicyHardwareKeyPIN, keys.PrivateKeyPolicyHardwareKeyTouchAndPIN:
					// The user is prompted to enter their PIN in terminal.
					redirectURL = terminalRedirectURL
				}
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
		select {
		case rd.responseC <- response:
			redirectURL := successURL
			switch rd.PrivateKeyPolicy {
			case keys.PrivateKeyPolicyHardwareKey:
				// login should complete without user interaction, success.
			case keys.PrivateKeyPolicyHardwareKeyPIN:
				// The user is prompted to enter their PIN before this step,
				// so we can go straight to success screen.
			case keys.PrivateKeyPolicyHardwareKeyTouch, keys.PrivateKeyPolicyHardwareKeyTouchAndPIN:
				// The user is prompted to touch their hardware key after
				// this redirect, so display the terminal redirect screen.
				redirectURL = terminalRedirectURL
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
		case <-rd.context.Done():
			http.Redirect(w, r, errorURL, http.StatusFound)
		}
	})
}
