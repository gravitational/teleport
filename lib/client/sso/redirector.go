/*
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

package sso

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/saml"
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

	// SAMLSingleLogoutFailedRedirectURL is the default redirect URL when an error was encountered during SAML Single Logout.
	SAMLSingleLogoutFailedRedirectURL = "/web/msg/error/slo"

	// DefaultLoginURL is the default login page.
	DefaultLoginURL = "/web/login"

	// WebMFARedirect is the landing page for SSO MFA in the WebUI. The WebUI set up a listener
	// on this page in order to capture the SSO MFA response regardless of what page the challenge
	// was requested from.
	WebMFARedirect = "/web/sso_confirm"
)

// RedirectorConfig is configuration for an sso redirector.
type RedirectorConfig struct {
	// ProxyAddr is the Teleport proxy address to use as the base redirect address
	// at the end of an SSO ceremony. e.g. https://<proxy_addr>/web/msg/info/login_success
	// required.
	ProxyAddr string

	// BindAddr is an optional host:port address to bind the local callback listener
	// to instead of localhost:<random_port>
	BindAddr string
	// CallbackAddr is an optional base URL to use as a callback address for the
	// local callback listener instead of localhost. If supplied, BindAddr must
	// be set to match it.
	CallbackAddr string
	// Browser can be used to pass the name of a browser to override the system
	// default (not currently implemented), or set to 'none' to suppress
	// browser opening entirely.
	Browser string
	// PrivateKeyPolicy is a key policy to follow during login.
	PrivateKeyPolicy keys.PrivateKeyPolicy
	// ConnectorDisplayName is an optional display name which may be used in some
	// redirect URL pages.
	ConnectorDisplayName string
	// Stderr is used output a clickable redirect URL for the user to complete login.
	Stderr io.Writer
}

// Redirector handles SSH redirect flow with the Teleport server
type Redirector struct {
	RedirectorConfig

	server *httptest.Server
	mux    *http.ServeMux

	// ClientCallbackURL is set once the redirector's local http server is running.
	ClientCallbackURL string

	// key is a secret key used to encode/decode
	// the data with the server, it is used so that other
	// programs running on the same computer can't easilly sniff
	// the data
	key secret.Key
	// responseC is a channel to receive responses
	responseC chan *authclient.SSHLoginResponse
	// errorC will contain errors
	errorC chan error
	// doneC will be closed when the redirector is closed.
	doneC chan struct{}
	// proxyURL is a URL to the Teleport Proxy
	proxyURL *url.URL
}

// NewRedirector returns new local web server redirector
func NewRedirector(config RedirectorConfig) (*Redirector, error) {
	if config.ProxyAddr == "" {
		return nil, trace.BadParameter("missing required field ProxyAddr")
	}

	// Add protocol if it's not present.
	proxyAddr := config.ProxyAddr
	if !strings.HasPrefix(proxyAddr, "https://") && !strings.HasPrefix(proxyAddr, "http://") {
		proxyAddr = "https://" + proxyAddr
	}

	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err, "'%v' is not a valid proxy address", config.ProxyAddr)
	}

	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}

	// Create secret key that will be sent with the request and then used the
	// decrypt the response from the server.
	key, err := secret.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// parse and format CallbackAddr.
	if config.CallbackAddr != "" {
		callbackURL, err := apiutils.ParseURL(config.CallbackAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Default to HTTPS if no scheme is specified.
		// This will allow users to specify an insecure HTTP URL but
		// the backend will verify if the callback URL is allowed.
		if callbackURL.Scheme == "" {
			callbackURL.Scheme = "https"
		}
		config.CallbackAddr = callbackURL.String()
	}

	rd := &Redirector{
		RedirectorConfig: config,
		proxyURL:         proxyURL,
		mux:              http.NewServeMux(),
		key:              key,
		responseC:        make(chan *authclient.SSHLoginResponse, 1),
		errorC:           make(chan error, 1),
		doneC:            make(chan struct{}),
	}

	// callback is a callback URL communicated to the Teleport proxy,
	// after SAML/OIDC login, the teleport will redirect user's browser
	// to this laptop-local URL
	rd.mux.Handle("/callback", rd.wrapCallback(rd.callback))

	if err := rd.startServer(); err != nil {
		return nil, trace.Wrap(err)
	}

	return rd, nil
}

// startServer starts an http server to handle the sso client callback.
func (rd *Redirector) startServer() error {
	if rd.BindAddr != "" {
		slog.DebugContext(context.Background(), "Binding to provided bind address.", "addr", rd.BindAddr)
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

	// Prepare callback URL.
	u, err := url.Parse(rd.baseURL() + "/callback")
	if err != nil {
		return trace.Wrap(err)
	}
	u.RawQuery = url.Values{"secret_key": {rd.key.String()}}.Encode()
	rd.ClientCallbackURL = u.String()

	return nil
}

// OpenRedirect opens the redirect URL in a new browser window.
func (rd *Redirector) OpenRedirect(ctx context.Context, redirectURL string) error {
	return trace.Wrap(rd.processLoginURL(redirectURL, ""))
}

// OpenLoginURL opens the redirector served redirect URL in a new browser window, suitable for
// both SAML http-redirect and http-post binding SSO authentication request.
func (rd *Redirector) OpenLoginURL(ctx context.Context, redirectURL, postForm string) error {
	return trace.Wrap(rd.processLoginURL(redirectURL, postForm))
}

func (rd *Redirector) processLoginURL(redirectURL, postForm string) error {
	if redirectURL == "" && postForm == "" {
		// This is not expected as either one of the param will always be populated
		// but we should return with an error to indicate a bug.
		return trace.BadParameter("either redirectURL or postForm value must be configured")
	}
	clickableURL := rd.clickableURL(redirectURL, postForm)

	// If a command was found to launch the browser, create and start it.
	if err := OpenURLInBrowser(rd.Browser, clickableURL); err != nil {
		fmt.Fprintf(rd.Stderr, "Failed to open a browser window for login: %v\n", err)
	}

	// Print the URL to the screen, in case the command that launches the browser did not run.
	// If Browser is set to the special string teleport.BrowserNone, no browser will be opened.
	if rd.Browser == teleport.BrowserNone {
		fmt.Fprintf(rd.Stderr, "Use the following URL to authenticate:\n %v\n", clickableURL)
	} else {
		fmt.Fprintf(rd.Stderr, "If browser window does not open automatically, open it by ")
		fmt.Fprintf(rd.Stderr, "clicking on the link:\n %v\n", clickableURL)
	}

	return nil
}

// clickableURL returns a short, clickable URL that will redirect
// the browser to the SSO redirect URL.
func (rd *Redirector) clickableURL(redirectURL, postForm string) string {
	// shortPath is a link-shortener path presented to the user
	// it is used to open up the browser window, notice
	// that redirectURL will be set later
	shortPath := "/" + uuid.New().String()

	// short path is a link-shortener style URL
	// that will redirect to the Teleport-Proxy supplied address
	rd.mux.HandleFunc(shortPath, func(w http.ResponseWriter, r *http.Request) {
		if postForm != "" {
			form, err := base64.StdEncoding.DecodeString(postForm)
			if err != nil {
				http.Error(w, err.Error(), trace.ErrorToCode(err))
				return
			}
			if err := saml.WriteSAMLPostRequestWithHeaders(w, form); err != nil {
				http.Error(w, err.Error(), trace.ErrorToCode(err))
				return
			}
		} else {
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	})

	return utils.ClickableURL(rd.baseURL() + shortPath)
}

func (rd *Redirector) baseURL() string {
	if rd.CallbackAddr != "" {
		return rd.CallbackAddr
	}
	return rd.server.URL
}

// OpenURLInBrowser opens a URL in a web browser.
func OpenURLInBrowser(browser string, URL string) error {
	var execCmd *exec.Cmd
	if browser != teleport.BrowserNone {
		switch runtime.GOOS {
		// macOS.
		case constants.DarwinOS:
			path, err := exec.LookPath(teleport.OpenBrowserDarwin)
			if err == nil {
				execCmd = exec.Command(path, URL)
			}
		// Windows.
		case constants.WindowsOS:
			path, err := exec.LookPath(teleport.OpenBrowserWindows)
			if err == nil {
				execCmd = exec.Command(path, "url.dll,FileProtocolHandler", URL)
			}
		// Linux or any other operating system.
		default:
			path, err := exec.LookPath(teleport.OpenBrowserLinux)
			if err == nil {
				execCmd = exec.Command(path, URL)
			}
		}
	}
	if execCmd != nil {
		if err := execCmd.Start(); err != nil {
			return err
		}
	}

	return nil
}

// WaitForResponse waits for a response from the callback handler.
func (rd *Redirector) WaitForResponse(ctx context.Context) (*authclient.SSHLoginResponse, error) {
	slog.InfoContext(ctx, "Waiting for response", "callback_url", rd.server.URL)
	select {
	case err := <-rd.ErrorC():
		slog.DebugContext(ctx, "Got an error", "err", err)
		return nil, trace.Wrap(err)
	case response := <-rd.ResponseC():
		slog.DebugContext(ctx, "Got response from browser.")
		return response, nil
	case <-time.After(defaults.SSOCallbackTimeout):
		slog.DebugContext(ctx, "Timed out waiting for callback", "timeout", defaults.SSOCallbackTimeout)
		return nil, trace.Wrap(trace.Errorf("timed out waiting for callback"))
	case <-rd.Done():
		slog.DebugContext(ctx, "Redirector closed")
		return nil, trace.Errorf("redirector closed")
	case <-ctx.Done():
		slog.DebugContext(ctx, "Canceled by user.")
		return nil, trace.Wrap(ctx.Err(), "canceled by user")
	}
}

// Done is called when redirector is closed
// or parent context is closed
func (rd *Redirector) Done() <-chan struct{} {
	return rd.doneC
}

// ResponseC returns a channel with response
func (rd *Redirector) ResponseC() <-chan *authclient.SSHLoginResponse {
	return rd.responseC
}

// ErrorC returns a channel with error
func (rd *Redirector) ErrorC() <-chan error {
	return rd.errorC
}

// callback is used by Teleport proxy to send back credentials
// issued by Teleport proxy
func (rd *Redirector) callback(w http.ResponseWriter, r *http.Request) (*authclient.SSHLoginResponse, error) {
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

	var re authclient.SSHLoginResponse
	err = json.Unmarshal(plaintext, &re)
	if err != nil {
		return nil, trace.BadParameter("failed to decrypt response: in %v, err: %v", r.URL.String(), err)
	}

	return &re, nil
}

// Close closes redirector and releases all resources
func (rd *Redirector) Close() {
	close(rd.doneC)
	rd.server.Close()
}

// wrapCallback is a helper wrapper method that wraps callback HTTP handler
// and sends a result to the channel and redirect users to error page
func (rd *Redirector) wrapCallback(fn func(http.ResponseWriter, *http.Request) (*authclient.SSHLoginResponse, error)) http.Handler {
	// Generate possible redirect URLs from the proxy URL.
	clone := *rd.proxyURL
	clone.Path = LoginFailedRedirectURL
	errorURL := clone.String()
	clone.Path = LoginSuccessRedirectURL
	successURL := clone.String()
	clone.Path = LoginClose
	closeURL := clone.String()

	clone.Path = LoginTerminalRedirectURL
	if rd.ConnectorDisplayName != "" {
		query := clone.Query()
		query.Set("auth", rd.ConnectorDisplayName)
		clone.RawQuery = query.Encode()
	}
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
			case <-rd.Done():
			}
			redirectURL := errorURL
			// A second SSO login attempt will be initiated if a key policy requirement was not satisfied.
			if requiredPolicy, err := keys.ParsePrivateKeyPolicyError(err); err == nil {
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
		case <-rd.Done():
			http.Redirect(w, r, errorURL, http.StatusFound)
		}
	})
}
