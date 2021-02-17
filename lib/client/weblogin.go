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
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
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

// CreateSSHCertWithMFAReq are passed by web client
// to authenticate against teleport server and receive
// a temporary cert signed by auth server authority
type CreateSSHCertWithMFAReq struct {
	// User is a teleport username
	User string `json:"user"`
	// Password for the user, to authenticate in case no MFA check was
	// performed.
	Password string `json:"password"`

	// U2FSignResponse is the signature from the U2F device
	U2FSignResponse *u2f.AuthenticateChallengeResponse `json:"u2f_sign_response"`
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

// PingResponse contains data about the Teleport server like supported
// authentication types, server version, etc.
type PingResponse struct {
	// Auth contains the forms of authentication the auth server supports.
	Auth AuthenticationSettings `json:"auth"`
	// Proxy contains the proxy settings.
	Proxy ProxySettings `json:"proxy"`
	// ServerVersion is the version of Teleport that is running.
	ServerVersion string `json:"server_version"`
	// MinClientVersion is the minimum client version required by the server.
	MinClientVersion string `json:"min_client_version"`
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
	// User is the login password.
	Password string
}

// ProxySettings contains basic information about proxy settings
type ProxySettings struct {
	// Kube is a kubernetes specific proxy section
	Kube KubeProxySettings `json:"kube"`
	// SSH is SSH specific proxy settings
	SSH SSHProxySettings `json:"ssh"`
	// DB contains database access specific proxy settings
	DB DBProxySettings `json:"db"`
}

// KubeProxySettings is kubernetes proxy settings
type KubeProxySettings struct {
	// Enabled is true when kubernetes proxy is enabled
	Enabled bool `json:"enabled,omitempty"`
	// PublicAddr is a kubernetes proxy public address if set
	PublicAddr string `json:"public_addr,omitempty"`
	// ListenAddr is the address that the kubernetes proxy is listening for
	// connections on.
	ListenAddr string `json:"listen_addr,omitempty"`
}

// SSHProxySettings is SSH specific proxy settings.
type SSHProxySettings struct {
	// ListenAddr is the address that the SSH proxy is listening for
	// connections on.
	ListenAddr string `json:"listen_addr,omitempty"`

	// TunnelListenAddr
	TunnelListenAddr string `json:"tunnel_listen_addr,omitempty"`

	// PublicAddr is the public address of the HTTP proxy.
	PublicAddr string `json:"public_addr,omitempty"`

	// SSHPublicAddr is the public address of the SSH proxy.
	SSHPublicAddr string `json:"ssh_public_addr,omitempty"`

	// TunnelPublicAddr is the public address of the SSH reverse tunnel.
	TunnelPublicAddr string `json:"ssh_tunnel_public_addr,omitempty"`
}

// DBProxySettings contains database access specific proxy settings.
type DBProxySettings struct {
	// MySQLListenAddr is MySQL proxy listen address.
	MySQLListenAddr string `json:"mysql_listen_addr,omitempty"`
}

// PingResponse contains the form of authentication the auth server supports.
type AuthenticationSettings struct {
	// Type is the type of authentication, can be either local or oidc.
	Type string `json:"type"`
	// SecondFactor is the type of second factor to use in authentication.
	// Supported options are: off, otp, and u2f.
	SecondFactor constants.SecondFactorType `json:"second_factor,omitempty"`
	// U2F contains the Universal Second Factor settings needed for authentication.
	U2F *U2FSettings `json:"u2f,omitempty"`
	// OIDC contains OIDC connector settings needed for authentication.
	OIDC *OIDCSettings `json:"oidc,omitempty"`
	// SAML contains SAML connector settings needed for authentication.
	SAML *SAMLSettings `json:"saml,omitempty"`
	// Github contains Github connector settings needed for authentication.
	Github *GithubSettings `json:"github,omitempty"`
}

// U2FSettings contains the AppID for Universal Second Factor.
type U2FSettings struct {
	// AppID is the U2F AppID.
	AppID string `json:"app_id"`
}

// SAMLSettings contains the Name and Display string for SAML
type SAMLSettings struct {
	// Name is the internal name of the connector.
	Name string `json:"name"`
	// Display is the display name for the connector.
	Display string `json:"display"`
}

// OIDCSettings contains the Name and Display string for OIDC.
type OIDCSettings struct {
	// Name is the internal name of the connector.
	Name string `json:"name"`
	// Display is the display name for the connector.
	Display string `json:"display"`
}

// GithubSettings contains the Name and Display string for Github connector.
type GithubSettings struct {
	// Name is the internal name of the connector
	Name string `json:"name"`
	// Display is the connector display name
	Display string `json:"display"`
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
		fmt.Printf("WARNING: You are using insecure connection to SSH proxy %v\n", proxyAddr)
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

// Ping serves two purposes. The first is to validate the HTTP endpoint of a
// Teleport proxy. This leads to better user experience: users get connection
// errors before being asked for passwords. The second is to return the form
// of authentication that the server supports. This also leads to better user
// experience: users only get prompted for the type of authentication the server supports.
func Ping(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool, connectorName string) (*PingResponse, error) {
	clt, _, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoint := clt.Endpoint("webapi", "ping")
	if connectorName != "" {
		endpoint = clt.Endpoint("webapi", "ping", connectorName)
	}

	response, err := clt.Get(ctx, endpoint, url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var pr *PingResponse
	err = json.Unmarshal(response.Bytes(), &pr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pr, nil
}

// Find is like ping, but used by servers to only fetch discovery data,
// without auth connector data, it is designed for servers in IOT mode
// to fetch proxy public addresses on a large scale.
func Find(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool) (*PingResponse, error) {
	clt, _, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := clt.Get(ctx, clt.Endpoint("webapi", "find"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var pr *PingResponse
	err = json.Unmarshal(response.Bytes(), &pr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return pr, nil
}

// SSHAgentSSOLogin is used by tsh to fetch user credentials using OpenID Connect (OIDC) or SAML.
func SSHAgentSSOLogin(ctx context.Context, login SSHLoginSSO) (*auth.SSHLoginResponse, error) {
	rd, err := NewRedirector(ctx, login)
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
		case teleport.DarwinOS:
			path, err := exec.LookPath(teleport.OpenBrowserDarwin)
			if err == nil {
				execCmd = exec.Command(path, clickableURL)
			}
		// Windows.
		case teleport.WindowsOS:
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
			fmt.Printf("Failed to open a browser window for login: %v\n", err)
		}
	}

	// Print the URL to the screen, in case the command that launches the browser did not run.
	// If Browser is set to the special string teleport.BrowserNone, no browser will be opened.
	if login.Browser == teleport.BrowserNone {
		fmt.Printf("Use the following URL to authenticate:\n %v\n", clickableURL)
	} else {
		fmt.Printf("If browser window does not open automatically, open it by ")
		fmt.Printf("clicking on the link:\n %v\n", clickableURL)
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
		return nil, trace.Wrap(ctx.Err(), "cancelled by user")
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

// SSHAgentMFALogin requests a MFA challenge (U2F or OTP) via the proxy. If the
// credentials are valid, the proxy wiil return a challenge. We then prompt the
// user to provide 2nd factor and pass the response to the proxy. If the
// authentication succeeds, we will get a temporary certificate back.
func SSHAgentMFALogin(ctx context.Context, login SSHLoginMFA) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(awly): mfa: rename endpoint
	chalRaw, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "u2f", "signrequest"), MFAChallengeRequest{
		User: login.User,
		Pass: login.Password,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var chal auth.MFAAuthenticateChallenge
	if err := json.Unmarshal(chalRaw.Bytes(), &chal); err != nil {
		return nil, trace.Wrap(err)
	}
	if len(chal.U2FChallenges) == 0 && chal.AuthenticateChallenge != nil {
		// Challenge sent by a pre-6.0 auth server, fall back to the old
		// single-device format.
		chal.U2FChallenges = []u2f.AuthenticateChallenge{*chal.AuthenticateChallenge}
	}

	// Convert to auth gRPC proto challenge.
	protoChal := new(proto.MFAAuthenticateChallenge)
	if chal.TOTPChallenge {
		protoChal.TOTP = new(proto.TOTPChallenge)
	}
	for _, u2fChal := range chal.U2FChallenges {
		protoChal.U2F = append(protoChal.U2F, &proto.U2FChallenge{
			KeyHandle: u2fChal.KeyHandle,
			Challenge: u2fChal.Challenge,
			AppID:     u2fChal.AppID,
		})
	}

	protoResp, err := PromptMFAChallenge(ctx, login.ProxyAddr, protoChal, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	chalResp := CreateSSHCertWithMFAReq{
		User:              login.User,
		Password:          login.Password,
		PubKey:            login.PubKey,
		TTL:               login.TTL,
		Compatibility:     login.Compatibility,
		RouteToCluster:    login.RouteToCluster,
		KubernetesCluster: login.KubernetesCluster,
	}
	// Convert back from auth gRPC proto response.
	switch r := protoResp.Response.(type) {
	case *proto.MFAAuthenticateResponse_TOTP:
		chalResp.TOTPCode = r.TOTP.Code
	case *proto.MFAAuthenticateResponse_U2F:
		chalResp.U2FSignResponse = &u2f.AuthenticateChallengeResponse{
			KeyHandle:     r.U2F.KeyHandle,
			SignatureData: r.U2F.Signature,
			ClientData:    r.U2F.ClientData,
		}
	default:
		// No challenge was sent, so we send back just username/password.
	}

	loginRespRaw, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "u2f", "certs"), chalResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var loginResp *auth.SSHLoginResponse
	err = json.Unmarshal(loginRespRaw.Bytes(), &loginResp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return loginResp, nil
}

// HostCredentials is used to fetch host credentials for a node.
func HostCredentials(ctx context.Context, proxyAddr string, insecure bool, req auth.RegisterUsingTokenRequest) (*auth.PackedKeys, error) {
	clt, _, err := initClient(proxyAddr, insecure, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "host", "credentials"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var packedKeys *auth.PackedKeys
	err = json.Unmarshal(resp.Bytes(), &packedKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return packedKeys, nil
}
