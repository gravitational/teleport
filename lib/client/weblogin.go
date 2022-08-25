/*
Copyright 2015-2019 Gravitational, Inc.

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

package client

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
)

const (
	// HTTPS is https prefix
	HTTPS = "https"
	// WSS is secure web sockets prefix
	WSS = "wss"
)

// SSOLoginConsoleReq is used to SSO for tsh
type SSOLoginConsoleReq struct {
	RedirectURL   string        `json:"redirect_url"`
	PublicKey     []byte        `json:"public_key"`
	CertTTL       time.Duration `json:"cert_ttl"`
	ConnectorID   string        `json:"connector_id"`
	Compatibility string        `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// CheckAndSetDefaults makes sure that the request is valid
func (r *SSOLoginConsoleReq) CheckAndSetDefaults() error {
	if r.RedirectURL == "" {
		return trace.BadParameter("missing RedirectURL")
	}
	if len(r.PublicKey) == 0 {
		return trace.BadParameter("missing PublicKey")
	}
	if r.ConnectorID == "" {
		return trace.BadParameter("missing ConnectorID")
	}
	return nil
}

// SSOLoginConsoleResponse is a response to SSO console request
type SSOLoginConsoleResponse struct {
	RedirectURL string `json:"redirect_url"`
}

// MFAChallengeRequest is a request from the client for a MFA challenge from the
// server.
type MFAChallengeRequest struct {
	User string `json:"user"`
	Pass string `json:"pass"`
	// Passwordless explicitly requests a passwordless/usernameless challenge.
	Passwordless bool `json:"passwordless"`
}

// CreateSSHCertReq are passed by web client
// to authenticate against teleport server and receive
// a temporary cert signed by auth server authority
type CreateSSHCertReq struct {
	// User is a teleport username
	User string `json:"user"`
	// Password is user's pass
	Password string `json:"password"`
	// HOTPToken is second factor token
	// Deprecated: HOTPToken is deprecated, use OTPToken.
	HOTPToken string `json:"hotp_token"`
	// OTPToken is second factor token
	OTPToken string `json:"otp_token"`
	// PubKey is a public key user wishes to sign
	PubKey []byte `json:"pub_key"`
	// TTL is a desired TTL for the cert (max is still capped by server,
	// however user can shorten the time)
	TTL time.Duration `json:"ttl"`
	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// AuthenticateSSHUserRequest are passed by web client to authenticate against
// teleport server and receive a temporary cert signed by auth server authority.
type AuthenticateSSHUserRequest struct {
	// User is a teleport username
	User string `json:"user"`
	// Password for the user, to authenticate in case no MFA check was
	// performed.
	Password string `json:"password"`
	// WebauthnChallengeResponse is a signed WebAuthn credential assertion.
	WebauthnChallengeResponse *wanlib.CredentialAssertionResponse `json:"webauthn_challenge_response"`
	// TOTPCode is a code from the TOTP device.
	TOTPCode string `json:"totp_code"`
	// PubKey is a public key user wishes to sign
	PubKey []byte `json:"pub_key"`
	// TTL is a desired TTL for the cert (max is still capped by server,
	// however user can shorten the time)
	TTL time.Duration `json:"ttl"`
	// Compatibility specifies OpenSSH compatibility flags.
	Compatibility string `json:"compatibility,omitempty"`
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

type AuthenticateWebUserRequest struct {
	// User is a teleport username.
	User string `json:"user"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	WebauthnAssertionResponse *wanlib.CredentialAssertionResponse `json:"webauthnAssertionResponse,omitempty"`
}

// SSHLogin contains common SSH login parameters.
type SSHLogin struct {
	// ProxyAddr is the target proxy address
	ProxyAddr string
	// PubKey is SSH public key to sign
	PubKey []byte
	// TTL is requested TTL of the client certificates
	TTL time.Duration
	// Insecure turns off verification for x509 target proxy
	Insecure bool
	// Pool is x509 cert pool to use for server certifcate verification
	Pool *x509.CertPool
	// Compatibility sets compatibility mode for SSH certificates
	Compatibility string
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
}

// SSHLoginSSO contains SSH login parameters for SSO login.
type SSHLoginSSO struct {
	SSHLogin
	// ConnectorID is the OIDC or SAML connector ID to use
	ConnectorID string
	// Protocol is an optional protocol selection
	Protocol string
	// BindAddr is an optional host:port address to bind
	// to for SSO login flows
	BindAddr string
	// Browser can be used to pass the name of a browser to override the system
	// default (not currently implemented), or set to 'none' to suppress
	// browser opening entirely.
	Browser string
}

// SSHLoginDirect contains SSH login parameters for direct (user/pass/OTP)
// login.
type SSHLoginDirect struct {
	SSHLogin
	// User is the login username.
	User string
	// User is the login password.
	Password string
	// User is the optional OTP token for the login.
	OTPToken string
}

// SSHLoginMFA contains SSH login parameters for MFA login.
type SSHLoginMFA struct {
	SSHLogin
	// User is the login username.
	User string
	// Password is the login password.
	Password string

	// AllowStdinHijack allows stdin hijack during MFA prompts.
	// Do not set this options unless you deeply understand what you are doing.
	AllowStdinHijack bool
	// AuthenticatorAttachment is the authenticator attachment for MFA prompts.
	AuthenticatorAttachment wancli.AuthenticatorAttachment
	// PreferOTP prefers OTP in favor of other MFA methods.
	PreferOTP bool
}

// initClient creates a new client to the HTTPS web proxy.
func initClient(proxyAddr string, insecure bool, pool *x509.CertPool) (*WebClient, *url.URL, error) {
	log := logrus.WithFields(logrus.Fields{
		trace.Component: teleport.ComponentClient,
	})
	log.Debugf("HTTPS client init(proxyAddr=%v, insecure=%v)", proxyAddr, insecure)

	// validate proxyAddr:
	host, port, err := net.SplitHostPort(proxyAddr)
	if err != nil || host == "" || port == "" {
		if err != nil {
			log.Error(err)
		}
		return nil, nil, trace.BadParameter("'%v' is not a valid proxy address", proxyAddr)
	}
	proxyAddr = "https://" + net.JoinHostPort(host, port)
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, nil, trace.BadParameter("'%v' is not a valid proxy address", proxyAddr)
	}

	var opts []roundtrip.ClientParam

	if insecure {
		// Skip https cert verification, print a warning that this is insecure.
		fmt.Fprintf(os.Stderr, "WARNING: You are using insecure connection to SSH proxy %v\n", proxyAddr)
		opts = append(opts, roundtrip.HTTPClient(NewInsecureWebClient()))
	} else if pool != nil {
		// use custom set of trusted CAs
		opts = append(opts, roundtrip.HTTPClient(newClientWithPool(pool)))
	}

	clt, err := NewWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clt, u, nil
}

// SSHAgentSSOLogin is used by tsh to fetch user credentials using OpenID Connect (OIDC) or SAML.
func SSHAgentSSOLogin(ctx context.Context, login SSHLoginSSO, config *RedirectorConfig) (*auth.SSHLoginResponse, error) {
	rd, err := NewRedirector(ctx, login, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := rd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer rd.Close()

	clickableURL := rd.ClickableURL()

	// If a command was found to launch the browser, create and start it.
	var execCmd *exec.Cmd
	if login.Browser != teleport.BrowserNone {
		switch runtime.GOOS {
		// macOS.
		case constants.DarwinOS:
			path, err := exec.LookPath(teleport.OpenBrowserDarwin)
			if err == nil {
				execCmd = exec.Command(path, clickableURL)
			}
		// Windows.
		case constants.WindowsOS:
			path, err := exec.LookPath(teleport.OpenBrowserWindows)
			if err == nil {
				execCmd = exec.Command(path, "url.dll,FileProtocolHandler", clickableURL)
			}
		// Linux or any other operating system.
		default:
			path, err := exec.LookPath(teleport.OpenBrowserLinux)
			if err == nil {
				execCmd = exec.Command(path, clickableURL)
			}
		}
	}
	if execCmd != nil {
		if err := execCmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open a browser window for login: %v\n", err)
		}
	}

	// Print the URL to the screen, in case the command that launches the browser did not run.
	// If Browser is set to the special string teleport.BrowserNone, no browser will be opened.
	if login.Browser == teleport.BrowserNone {
		fmt.Fprintf(os.Stderr, "Use the following URL to authenticate:\n %v\n", clickableURL)
	} else {
		fmt.Fprintf(os.Stderr, "If browser window does not open automatically, open it by ")
		fmt.Fprintf(os.Stderr, "clicking on the link:\n %v\n", clickableURL)
	}

	select {
	case err := <-rd.ErrorC():
		log.Debugf("Got an error: %v.", err)
		return nil, trace.Wrap(err)
	case response := <-rd.ResponseC():
		log.Debugf("Got response from browser.")
		return response, nil
	case <-time.After(defaults.CallbackTimeout):
		log.Debugf("Timed out waiting for callback after %v.", defaults.CallbackTimeout)
		return nil, trace.Wrap(trace.Errorf("timed out waiting for callback"))
	case <-rd.Done():
		log.Debugf("Canceled by user.")
		return nil, trace.Wrap(ctx.Err(), "canceled by user")
	}
}

// SSHAgentLogin is used by tsh to fetch local user credentials.
func SSHAgentLogin(ctx context.Context, login SSHLoginDirect) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), CreateSSHCertReq{
		User:              login.User,
		Password:          login.Password,
		OTPToken:          login.OTPToken,
		PubKey:            login.PubKey,
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out *auth.SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return out, nil
}

// SSHAgentMFALogin requests a MFA challenge via the proxy.
// If the credentials are valid, the proxy will return a challenge. We then
// prompt the user to provide 2nd factor and pass the response to the proxy.
// If the authentication succeeds, we will get a temporary certificate back.
func SSHAgentMFALogin(ctx context.Context, login SSHLoginMFA) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	beginReq := MFAChallengeRequest{
		User: login.User,
		Pass: login.Password,
	}
	challengeJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), beginReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, trace.Wrap(err)
	}

	// Convert to auth gRPC proto challenge.
	challengePB := &proto.MFAAuthenticateChallenge{}
	if challenge.TOTPChallenge {
		challengePB.TOTP = &proto.TOTPChallenge{}
	}
	if challenge.WebauthnChallenge != nil {
		challengePB.WebauthnChallenge = wanlib.CredentialAssertionToProto(challenge.WebauthnChallenge)
	}

	respPB, err := PromptMFAChallenge(ctx, challengePB, login.ProxyAddr, &PromptMFAChallengeOpts{
		AllowStdinHijack:        login.AllowStdinHijack,
		AuthenticatorAttachment: login.AuthenticatorAttachment,
		PreferOTP:               login.PreferOTP,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeResp := AuthenticateSSHUserRequest{
		User:              login.User,
		Password:          login.Password,
		PubKey:            login.PubKey,
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	}
	// Convert back from auth gRPC proto response.
	switch r := respPB.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		challengeResp.TOTPCode = r.TOTP.Code
	case *proto.MFAAuthenticateResponse_Webauthn:
		challengeResp.WebauthnChallengeResponse = wanlib.CredentialAssertionResponseFromProto(r.Webauthn)
	default:
		// No challenge was sent, so we send back just username/password.
	}

	loginRespJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finish"), challengeResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginResp := &auth.SSHLoginResponse{}
	return loginResp, trace.Wrap(json.Unmarshal(loginRespJSON.Bytes(), loginResp))
}

// HostCredentials is used to fetch host credentials for a node.
func HostCredentials(ctx context.Context, proxyAddr string, insecure bool, req types.RegisterUsingTokenRequest) (*proto.Certs, error) {
	clt, _, err := initClient(proxyAddr, insecure, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.PostJSONWithFallback(ctx, clt.Endpoint("webapi", "host", "credentials"), req, insecure)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var certs proto.Certs
	if err := json.Unmarshal(resp.Bytes(), &certs); err != nil {
		return nil, trace.Wrap(err)
	}

	return &certs, nil
}

// GetWebConfig is used by teleterm to fetch webconfig.js from proxies
func GetWebConfig(ctx context.Context, proxyAddr string, insecure bool) (*webclient.WebConfig, error) {
	clt, _, err := initClient(proxyAddr, insecure, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.Get(ctx, clt.Endpoint("web", "config.js"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	body, err := io.ReadAll(response.Reader())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// WebConfig is served as JS file where GRV_CONFIG is a global object name
	text := bytes.TrimSuffix(bytes.Replace(body, []byte("var GRV_CONFIG = "), []byte(""), 1), []byte(";"))

	cfg := webclient.WebConfig{}
	if err := json.Unmarshal(text, &cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cfg, nil
}
