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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/auth"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/utils"
	websession "github.com/gravitational/teleport/lib/web/session"
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
	// AttestationStatement is an attestation statement associated with the given public key.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
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

// MFAChallengeResponse holds the response to a MFA challenge.
type MFAChallengeResponse struct {
	// TOTPCode is a code for a otp device.
	TOTPCode string `json:"totp_code,omitempty"`
	// WebauthnResponse is a response from a webauthn device.
	WebauthnResponse *wantypes.CredentialAssertionResponse `json:"webauthn_response,omitempty"`
}

// GetOptionalMFAResponseProtoReq converts response to a type proto.MFAAuthenticateResponse,
// if there were any responses set. Otherwise returns nil.
func (r *MFAChallengeResponse) GetOptionalMFAResponseProtoReq() (*proto.MFAAuthenticateResponse, error) {
	if r.TOTPCode != "" && r.WebauthnResponse != nil {
		return nil, trace.BadParameter("only one MFA response field can be set")
	}

	if r.TOTPCode != "" {
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: r.TOTPCode},
		}}, nil
	}

	if r.WebauthnResponse != nil {
		return &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wantypes.CredentialAssertionResponseToProto(r.WebauthnResponse),
		}}, nil
	}

	return nil, nil
}

// CreateSSHCertReq are passed by web client
// to authenticate against teleport server and receive
// a temporary cert signed by auth server authority
type CreateSSHCertReq struct {
	// User is a teleport username
	User string `json:"user"`
	// Password is user's pass
	Password string `json:"password"`
	// OTPToken is second factor token
	OTPToken string `json:"otp_token"`
	// HeadlessAuthenticationID is a headless authentication resource id.
	HeadlessAuthenticationID string `json:"headless_id"`
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
	// AttestationStatement is an attestation statement associated with the given public key.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
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
	WebauthnChallengeResponse *wantypes.CredentialAssertionResponse `json:"webauthn_challenge_response"`
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
	// AttestationStatement is an attestation statement associated with the given public key.
	AttestationStatement *keys.AttestationStatement `json:"attestation_statement,omitempty"`
}

type AuthenticateWebUserRequest struct {
	// User is a teleport username.
	User string `json:"user"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse,omitempty"`
}

type HeadlessRequest struct {
	// Actions can be either accept or deny.
	Action string `json:"action"`
	// WebauthnAssertionResponse is a signed WebAuthn credential assertion.
	WebauthnAssertionResponse *wantypes.CredentialAssertionResponse `json:"webauthnAssertionResponse,omitempty"`
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
	// Pool is x509 cert pool to use for server certificate verification
	Pool *x509.CertPool
	// Compatibility sets compatibility mode for SSH certificates
	Compatibility string
	// RouteToCluster is an optional cluster name to route the response
	// credentials to.
	RouteToCluster string
	// KubernetesCluster is an optional k8s cluster name to route the response
	// credentials to.
	KubernetesCluster string
	// AttestationStatement is an attestation statement.
	AttestationStatement *keys.AttestationStatement
	// ExtraHeaders is a map of extra HTTP headers to be included in requests.
	ExtraHeaders map[string]string
}

// SSHLoginSSO contains SSH login parameters for SSO login.
type SSHLoginSSO struct {
	SSHLogin
	// ConnectorID is the OIDC or SAML connector ID to use
	ConnectorID string
	// ConnectorName is the display name of the connector.
	ConnectorName string
	// Protocol is an optional protocol selection
	Protocol string
	// BindAddr is an optional host:port address to bind
	// to for SSO login flows
	BindAddr string
	// CallbackAddr is the optional base URL to give to the user when performing
	// SSO redirect flows.
	CallbackAddr string
	// Browser can be used to pass the name of a browser to override the system
	// default (not currently implemented), or set to 'none' to suppress
	// browser opening entirely.
	Browser string
	// PrivateKeyPolicy is a key policy to follow during login.
	PrivateKeyPolicy keys.PrivateKeyPolicy
	// ProxySupportsKeyPolicyMessage lets the tsh redirector give users more
	// useful messages in the web UI if the proxy supports them.
	// TODO(atburke): DELETE in v17.0.0
	ProxySupportsKeyPolicyMessage bool
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
	// PromptMFA is a customizable MFA prompt function.
	// Defaults to [mfa.NewPrompt().Run]
	PromptMFA mfa.Prompt
	// User is the login username.
	User string
	// Password is the login password.
	Password string
}

// SSHLoginPasswordless contains SSH login parameters for passwordless login.
type SSHLoginPasswordless struct {
	SSHLogin

	// WebauthnLogin is a customizable webauthn login function.
	// Defaults to [wancli.Login]
	WebauthnLogin WebauthnLoginFunc

	// StderrOverride will override the default os.Stderr if provided.
	StderrOverride io.Writer

	// User is the login username.
	User string

	// AuthenticatorAttachment is the authenticator attachment for passwordless prompts.
	AuthenticatorAttachment wancli.AuthenticatorAttachment

	// CustomPrompt defines a custom webauthn login prompt.
	// It's an optional field that when nil, it will use the wancli.DefaultPrompt.
	CustomPrompt wancli.LoginPrompt
}

type SSHLoginHeadless struct {
	SSHLogin

	// User is the login username.
	User string

	// HeadlessAuthenticationID is a headless authentication request ID.
	HeadlessAuthenticationID string
}

// MFAAuthenticateChallenge is an MFA authentication challenge sent on user
// login / authentication ceremonies.
type MFAAuthenticateChallenge struct {
	// WebauthnChallenge contains a WebAuthn credential assertion used for
	// login/authentication ceremonies.
	WebauthnChallenge *wantypes.CredentialAssertion `json:"webauthn_challenge"`
	// TOTPChallenge specifies whether TOTP is supported for this user.
	TOTPChallenge bool `json:"totp_challenge"`
}

// MFARegisterChallenge is an MFA register challenge sent on new MFA register.
type MFARegisterChallenge struct {
	// Webauthn contains webauthn challenge.
	Webauthn *wantypes.CredentialCreation `json:"webauthn"`
	// TOTP contains TOTP challenge.
	TOTP *TOTPRegisterChallenge `json:"totp"`
}

// TOTPRegisterChallenge contains a TOTP challenge.
type TOTPRegisterChallenge struct {
	QRCode []byte `json:"qrCode"`
}

// initClient creates a new client to the HTTPS web proxy.
func initClient(proxyAddr string, insecure bool, pool *x509.CertPool, extraHeaders map[string]string, opts ...roundtrip.ClientParam) (*WebClient, *url.URL, error) {
	log := logrus.WithFields(logrus.Fields{
		teleport.ComponentKey: teleport.ComponentClient,
	})
	log.Debugf("HTTPS client init(proxyAddr=%v, insecure=%v, extraHeaders=%v)", proxyAddr, insecure, extraHeaders)

	// validate proxy address
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

	if insecure {
		// Skipping https cert verification, print a warning that this is insecure.
		fmt.Fprintf(os.Stderr, "WARNING: You are using insecure connection to Teleport proxy %v\n", proxyAddr)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	opts = append(opts,
		roundtrip.HTTPClient(newClient(insecure, pool, extraHeaders)),
		roundtrip.CookieJar(jar),
	)
	clt, err := NewWebClient(proxyAddr, opts...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clt, u, nil
}

// SSHAgentSSOLogin is used by tsh to fetch user credentials using OpenID Connect (OIDC) or SAML.
func SSHAgentSSOLogin(ctx context.Context, login SSHLoginSSO, config *RedirectorConfig) (*auth.SSHLoginResponse, error) {
	if login.CallbackAddr != "" && !utils.AsBool(os.Getenv("TELEPORT_LOGIN_SKIP_REMOTE_HOST_WARNING")) {
		const callbackPrompt = "Logging in from a remote host means that credentials will be stored on " +
			"the remote host. Make sure that you trust the provided callback host " +
			"(%v) and that it resolves to the provided bind addr (%v). Continue?"
		ok, err := prompt.Confirmation(ctx, os.Stderr, prompt.NewContextReader(os.Stdin),
			fmt.Sprintf(callbackPrompt, login.CallbackAddr, login.BindAddr),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !ok {
			return nil, trace.BadParameter("Login canceled.")
		}
	}
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
	case <-time.After(defaults.SSOCallbackTimeout):
		log.Debugf("Timed out waiting for callback after %v.", defaults.SSOCallbackTimeout)
		return nil, trace.Wrap(trace.Errorf("timed out waiting for callback"))
	case <-rd.Done():
		log.Debugf("Canceled by user.")
		return nil, trace.Wrap(ctx.Err(), "canceled by user")
	}
}

// SSHAgentLogin is used by tsh to fetch local user credentials.
func SSHAgentLogin(ctx context.Context, login SSHLoginDirect) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), CreateSSHCertReq{
		User:                 login.User,
		Password:             login.Password,
		OTPToken:             login.OTPToken,
		PubKey:               login.PubKey,
		TTL:                  login.TTL,
		Compatibility:        login.Compatibility,
		RouteToCluster:       login.RouteToCluster,
		KubernetesCluster:    login.KubernetesCluster,
		AttestationStatement: login.AttestationStatement,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out auth.SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// SSHAgentHeadlessLogin begins the headless login ceremony, returning new user certificates if successful.
func SSHAgentHeadlessLogin(ctx context.Context, login SSHLoginHeadless) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// This request will block until the headless login is approved.
	clt.Client.HTTPClient().Timeout = defaults.HeadlessLoginTimeout

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "ssh", "certs"), CreateSSHCertReq{
		User:                     login.User,
		HeadlessAuthenticationID: login.HeadlessAuthenticationID,
		PubKey:                   login.PubKey,
		TTL:                      login.TTL,
		Compatibility:            login.Compatibility,
		RouteToCluster:           login.RouteToCluster,
		KubernetesCluster:        login.KubernetesCluster,
		AttestationStatement:     login.AttestationStatement,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out auth.SSHLoginResponse
	err = json.Unmarshal(re.Bytes(), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &out, nil
}

// SSHAgentPasswordlessLogin requests a passwordless MFA challenge via the proxy.
// weblogin.CustomPrompt (or a default prompt) is used for interaction with the
// end user.
//
// Returns the SSH certificate if authn is successful or an error.
func SSHAgentPasswordlessLogin(ctx context.Context, login SSHLoginPasswordless) (*auth.SSHLoginResponse, error) {
	webClient, webURL, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"),
		&MFAChallengeRequest{
			Passwordless: true,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, trace.Wrap(err)
	}
	// Sanity check WebAuthn challenge.
	switch {
	case challenge.WebauthnChallenge == nil:
		return nil, trace.BadParameter("passwordless: webauthn challenge missing")
	case challenge.WebauthnChallenge.Response.UserVerification == protocol.VerificationDiscouraged:
		return nil, trace.BadParameter("passwordless: user verification requirement too lax (%v)", challenge.WebauthnChallenge.Response.UserVerification)
	}

	stderr := login.StderrOverride
	if stderr == nil {
		stderr = os.Stderr
	}

	prompt := login.CustomPrompt
	if prompt == nil {
		prompt = wancli.NewDefaultPrompt(ctx, stderr)
	}

	promptWebauthn := login.WebauthnLogin
	if promptWebauthn == nil {
		promptWebauthn = wancli.Login
	}

	mfaResp, _, err := promptWebauthn(ctx, webURL.String(), challenge.WebauthnChallenge, prompt, &wancli.LoginOpts{
		User:                    login.User,
		AuthenticatorAttachment: login.AuthenticatorAttachment,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginRespJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "finish"),
		&AuthenticateSSHUserRequest{
			User:                      "", // User carried on WebAuthn assertion.
			WebauthnChallengeResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
			PubKey:                    login.PubKey,
			TTL:                       login.TTL,
			Compatibility:             login.Compatibility,
			RouteToCluster:            login.RouteToCluster,
			KubernetesCluster:         login.KubernetesCluster,
			AttestationStatement:      login.AttestationStatement,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	loginResp := &auth.SSHLoginResponse{}
	if err := json.Unmarshal(loginRespJSON.Bytes(), loginResp); err != nil {
		return nil, trace.Wrap(err)
	}
	return loginResp, nil
}

// SSHAgentMFALogin requests a MFA challenge via the proxy.
// If the credentials are valid, the proxy will return a challenge. We then
// prompt the user to provide 2nd factor and pass the response to the proxy.
// If the authentication succeeds, we will get a temporary certificate back.
func SSHAgentMFALogin(ctx context.Context, login SSHLoginMFA) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
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
	chal := &proto.MFAAuthenticateChallenge{}
	if challenge.TOTPChallenge {
		chal.TOTP = &proto.TOTPChallenge{}
	}
	if challenge.WebauthnChallenge != nil {
		chal.WebauthnChallenge = wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge)
	}

	promptMFA := login.PromptMFA
	if promptMFA == nil {
		promptMFA = libmfa.NewCLIPrompt(libmfa.NewPromptConfig(login.ProxyAddr), os.Stderr)
	}

	respPB, err := promptMFA.Run(ctx, chal)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challengeResp := AuthenticateSSHUserRequest{
		User:                 login.User,
		Password:             login.Password,
		PubKey:               login.PubKey,
		TTL:                  login.TTL,
		Compatibility:        login.Compatibility,
		RouteToCluster:       login.RouteToCluster,
		KubernetesCluster:    login.KubernetesCluster,
		AttestationStatement: login.AttestationStatement,
	}
	// Convert back from auth gRPC proto response.
	switch r := respPB.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		challengeResp.TOTPCode = r.TOTP.Code
	case *proto.MFAAuthenticateResponse_Webauthn:
		challengeResp.WebauthnChallengeResponse = wantypes.CredentialAssertionResponseFromProto(r.Webauthn)
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
	clt, _, err := initClient(proxyAddr, insecure, nil, nil)
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
	clt, _, err := initClient(proxyAddr, insecure, nil, nil)
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

// CreateWebSessionReq is a request for the web api to
// initiate a new web session.
type CreateWebSessionReq struct {
	// User is the Teleport username.
	User string `json:"user"`
	// Pass is the password.
	Pass string `json:"pass"`
	// SecondFactorToken is the OTP.
	SecondFactorToken string `json:"second_factor_token"`
}

// CreateWebSessionResponse is a response from the web api
// to a [CreateWebSessionReq] request.
type CreateWebSessionResponse struct {
	// TokenType is token type (bearer)
	TokenType string `json:"type"`
	// Token value
	Token string `json:"token"`
	// TokenExpiresIn sets seconds before this token is not valid
	TokenExpiresIn int `json:"expires_in"`
	// SessionExpires is when this session expires.
	SessionExpires time.Time `json:"sessionExpires,omitempty"`
	// SessionInactiveTimeoutMS specifies how long in milliseconds
	// a user WebUI session can be left idle before being logged out
	// by the server. A zero value means there is no idle timeout set.
	SessionInactiveTimeoutMS int `json:"sessionInactiveTimeout"`
}

// SSHAgentLoginWeb is used by tsh to fetch local user credentials via the web api.
func SSHAgentLoginWeb(ctx context.Context, login SSHLoginDirect) (*WebClient, types.WebSession, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	csrfToken := hex.EncodeToString(token)
	resp, err := httplib.ConvertResponse(clt.RoundTrip(func() (*http.Response, error) {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(&CreateWebSessionReq{
			User:              login.User,
			Pass:              login.Password,
			SecondFactorToken: login.OTPToken,
		}); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", clt.Endpoint("webapi", "sessions", "web"), &buf)
		if err != nil {
			return nil, err
		}

		cookie := &http.Cookie{
			Name:  csrf.CookieName,
			Value: csrfToken,
		}

		req.AddCookie(cookie)

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrf.HeaderName, csrfToken)
		return clt.HTTPClient().Do(req)
	}))
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	session, err := GetSessionFromResponse(resp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return clt, session, nil
}

// SSHAgentMFAWebSessionLogin requests a MFA challenge via the proxy web api.
// If the credentials are valid, the proxy will return a challenge. We then
// prompt the user to provide 2nd factor and pass the response to the proxy.
func SSHAgentMFAWebSessionLogin(ctx context.Context, login SSHLoginMFA) (*WebClient, types.WebSession, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	beginReq := MFAChallengeRequest{
		User: login.User,
		Pass: login.Password,
	}
	challengeJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "begin"), beginReq)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Convert to auth gRPC proto challenge.
	chal := &proto.MFAAuthenticateChallenge{}
	if challenge.TOTPChallenge {
		chal.TOTP = &proto.TOTPChallenge{}
	}
	if challenge.WebauthnChallenge != nil {
		chal.WebauthnChallenge = wantypes.CredentialAssertionToProto(challenge.WebauthnChallenge)
	}

	promptMFA := login.PromptMFA
	if promptMFA == nil {
		promptMFA = libmfa.NewCLIPrompt(libmfa.NewPromptConfig(login.ProxyAddr), os.Stderr)
	}

	respPB, err := promptMFA.Run(ctx, chal)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	challengeResp := AuthenticateWebUserRequest{
		User: login.User,
	}
	// Convert back from auth gRPC proto response.
	switch r := respPB.Response.(type) {
	case *proto.MFAAuthenticateResponse_Webauthn:
		challengeResp.WebauthnAssertionResponse = wantypes.CredentialAssertionResponseFromProto(r.Webauthn)
	default:
		// No challenge was sent, so we send back just username/password.
	}

	loginRespJSON, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "mfa", "login", "finishsession"), challengeResp)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	session, err := GetSessionFromResponse(loginRespJSON)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return clt, session, nil
}

// SSHAgentPasswordlessLoginWeb requests a passwordless MFA challenge via the proxy
// web api.
func SSHAgentPasswordlessLoginWeb(ctx context.Context, login SSHLoginPasswordless) (*WebClient, types.WebSession, error) {
	webClient, webURL, err := initClient(login.ProxyAddr, login.Insecure, login.Pool, login.ExtraHeaders)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	challengeJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "begin"),
		&MFAChallengeRequest{
			Passwordless: true,
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	challenge := &MFAAuthenticateChallenge{}
	if err := json.Unmarshal(challengeJSON.Bytes(), challenge); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// Sanity check WebAuthn challenge.
	switch {
	case challenge.WebauthnChallenge == nil:
		return nil, nil, trace.BadParameter("passwordless: webauthn challenge missing")
	case challenge.WebauthnChallenge.Response.UserVerification == protocol.VerificationDiscouraged:
		return nil, nil, trace.BadParameter("passwordless: user verification requirement too lax (%v)", challenge.WebauthnChallenge.Response.UserVerification)
	}

	stderr := login.StderrOverride
	if stderr == nil {
		stderr = os.Stderr
	}

	prompt := login.CustomPrompt
	if prompt == nil {
		prompt = wancli.NewDefaultPrompt(ctx, stderr)
	}

	promptWebauthn := login.WebauthnLogin
	if promptWebauthn == nil {
		promptWebauthn = wancli.Login
	}

	mfaResp, _, err := promptWebauthn(ctx, webURL.String(), challenge.WebauthnChallenge, prompt, &wancli.LoginOpts{
		User:                    login.User,
		AuthenticatorAttachment: login.AuthenticatorAttachment,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	loginRespJSON, err := webClient.PostJSON(
		ctx, webClient.Endpoint("webapi", "mfa", "login", "finishsession"),
		&AuthenticateWebUserRequest{
			User:                      login.User,
			WebauthnAssertionResponse: wantypes.CredentialAssertionResponseFromProto(mfaResp.GetWebauthn()),
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	webSession, err := GetSessionFromResponse(loginRespJSON)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return webClient, webSession, nil
}

// GetSessionFromResponse creates a [types.WebSession] if a cookie
// named [websession.CookieName] is present in the provided [roundtrip.Response].
func GetSessionFromResponse(resp *roundtrip.Response) (types.WebSession, error) {
	var sess CreateWebSessionResponse
	if err := json.Unmarshal(resp.Bytes(), &sess); err != nil {
		return nil, trace.Wrap(err)
	}

	cookies := resp.Cookies()

	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == websession.CookieName {
			sessionCookie = cookie
			break
		}
	}

	if sessionCookie == nil {
		return nil, trace.BadParameter("no session cookie present")
	}

	cookie, err := websession.DecodeCookie(sessionCookie.Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := types.NewWebSession(cookie.SID, types.KindWebSession, types.WebSessionSpecV2{
		User:               cookie.User,
		BearerToken:        sess.Token,
		BearerTokenExpires: time.Now().Add(time.Duration(sess.TokenExpiresIn) * time.Second),
		Expires:            sess.SessionExpires,
		LoginTime:          time.Now(),
		IdleTimeout:        types.Duration(time.Duration(sess.SessionInactiveTimeoutMS) * time.Millisecond),
	})
	return session, trace.Wrap(err)
}
