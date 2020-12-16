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
	"io"
	"net"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
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

// U2fSignRequestReq is a request from the client for a U2F sign request from the server
type U2fSignRequestReq struct {
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

// CreateSSHCertWithU2FReq are passed by web client
// to authenticate against teleport server and receive
// a temporary cert signed by auth server authority
type CreateSSHCertWithU2FReq struct {
	// User is a teleport username
	User string `json:"user"`
	// We only issue U2F sign requests after checking the password, so there's no need to check again.
	// U2FSignResponse is the signature from the U2F device
	U2FSignResponse u2f.SignResponse `json:"u2f_sign_response"`
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

// SSHLoginU2F contains SSH login parameters for U2F login.
type SSHLoginU2F struct {
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
	SecondFactor string `json:"second_factor,omitempty"`
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

// SSHAgentU2FLogin requests a U2F sign request (authentication challenge) via
// the proxy. If the credentials are valid, the proxy wiil return a challenge.
// We then call the official u2f-host binary to perform the signing and pass
// the signature to the proxy. If the authentication succeeds, we will get a
// temporary certificate back.
func SSHAgentU2FLogin(ctx context.Context, login SSHLoginU2F) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(login.ProxyAddr, login.Insecure, login.Pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u2fSignRequest, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "u2f", "signrequest"), U2fSignRequestReq{
		User: login.User,
		Pass: login.Password,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pass the JSON-encoded data undecoded to the u2f-host binary
	facet := "https://" + strings.ToLower(login.ProxyAddr)
	cmd := exec.Command("u2f-host", "-aauthenticate", "-o", facet)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		// If we returned before cmd.Wait was called, clean up the spawned
		// process. ProcessState will be empty until cmd.Wait or cmd.Run
		// return.
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			cmd.Process.Kill()
		}
	}()
	_, err = stdin.Write(u2fSignRequest.Bytes())
	stdin.Close()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fmt.Println("Please press the button on your U2F key")

	// The origin URL is passed back base64-encoded and the keyHandle is passed back as is.
	// A very long proxy hostname or keyHandle can overflow a fixed-size buffer.
	signResponseLen := 500 + len(u2fSignRequest.Bytes()) + len(login.ProxyAddr)*4/3
	signResponseBuf := make([]byte, signResponseLen)
	signResponseLen, err = io.ReadFull(stdout, signResponseBuf)
	// unexpected EOF means we have read the data completely.
	if err == nil {
		return nil, trace.LimitExceeded("u2f sign response exceeded buffer size")
	}

	// Read error message (if any). 100 bytes is more than enough for any error message u2f-host outputs
	errMsgBuf := make([]byte, 100)
	errMsgLen, err := io.ReadFull(stderr, errMsgBuf)
	if err == nil {
		return nil, trace.LimitExceeded("u2f error message exceeded buffer size")
	}

	err = cmd.Wait()
	if err != nil {
		return nil, trace.AccessDenied("u2f-host returned error: " + string(errMsgBuf[:errMsgLen]))
	} else if signResponseLen == 0 {
		return nil, trace.NotFound("u2f-host returned no error and no sign response")
	}

	var u2fSignResponse *u2f.SignResponse
	err = json.Unmarshal(signResponseBuf[:signResponseLen], &u2fSignResponse)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	re, err := clt.PostJSON(ctx, clt.Endpoint("webapi", "u2f", "certs"), CreateSSHCertWithU2FReq{
		User:              login.User,
		U2FSignResponse:   *u2fSignResponse,
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
