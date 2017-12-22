/*
Copyright 2015 Gravitational, Inc.

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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/auth"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/mailgun/lemma/secret"
	log "github.com/sirupsen/logrus"

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
}

// Check makes sure that the request is valid
func (r SSOLoginConsoleReq) Check() error {
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

// A request from the client for a U2F sign request from the server
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
}

type sealData struct {
	Value []byte `json:"value"`
	Nonce []byte `json:"nonce"`
}

// SSHAgentSSOLogin is used by SSH Agent (tsh) to login using OpenID connect
func SSHAgentSSOLogin(proxyAddr, connectorID string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool, protocol string, compatibility string) (*auth.SSHLoginResponse, error) {
	clt, proxyURL, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create one time encoding secret that we will use to verify
	// callback from proxy that is received over untrusted channel (HTTP)
	keyBytes, err := secret.NewKey()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decryptor, err := secret.New(&secret.Config{KeyBytes: keyBytes})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	waitC := make(chan *auth.SSHLoginResponse, 1)
	errorC := make(chan error, 1)
	proxyURL.Path = "/web/msg/error/login_failed"
	redirectErrorURL := proxyURL.String()
	proxyURL.Path = "/web/msg/info/login_success"
	redirectSuccessURL := proxyURL.String()

	makeHandler := func(fn func(http.ResponseWriter, *http.Request) (*auth.SSHLoginResponse, error)) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response, err := fn(w, r)
			if err != nil {
				if trace.IsNotFound(err) {
					http.NotFound(w, r)
					return
				}
				errorC <- err
				http.Redirect(w, r, redirectErrorURL, http.StatusFound)
				return
			}
			waitC <- response
			http.Redirect(w, r, redirectSuccessURL, http.StatusFound)
		})
	}

	server := httptest.NewServer(makeHandler(func(w http.ResponseWriter, r *http.Request) (*auth.SSHLoginResponse, error) {
		if r.URL.Path != "/callback" {
			return nil, trace.NotFound("path not found")
		}
		encrypted := r.URL.Query().Get("response")
		if encrypted == "" {
			return nil, trace.BadParameter("missing required query parameters in %v", r.URL.String())
		}

		var encryptedData *secret.SealedBytes
		err := json.Unmarshal([]byte(encrypted), &encryptedData)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response in %v", r.URL.String())
		}

		out, err := decryptor.Open(encryptedData)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response: in %v, err: %v", r.URL.String(), err)
		}

		var re *auth.SSHLoginResponse
		err = json.Unmarshal([]byte(out), &re)
		if err != nil {
			return nil, trace.BadParameter("failed to decode response: in %v, err: %v", r.URL.String(), err)
		}
		return re, nil
	}))
	defer server.Close()

	u, err := url.Parse(server.URL + "/callback")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	query := u.Query()
	query.Set("secret", secret.KeyToEncodedString(keyBytes))
	u.RawQuery = query.Encode()

	out, err := clt.PostJSON(clt.Endpoint("webapi", protocol, "login", "console"), SSOLoginConsoleReq{
		RedirectURL:   u.String(),
		PublicKey:     pubKey,
		CertTTL:       ttl,
		ConnectorID:   connectorID,
		Compatibility: compatibility,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var re *SSOLoginConsoleResponse
	err = json.Unmarshal(out.Bytes(), &re)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fmt.Printf("If browser window does not open automatically, open it by clicking on the link:\n %v\n", re.RedirectURL)

	var command = "sensible-browser"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	path, err := exec.LookPath(command)
	if err == nil {
		exec.Command(path, re.RedirectURL).Start()
	}

	log.Infof("waiting for response on %v", server.URL)

	select {
	case err := <-errorC:
		log.Debugf("got error: %v", err)
		return nil, trace.Wrap(err)
	case response := <-waitC:
		log.Debugf("got response")
		return response, nil
	case <-time.After(60 * time.Second):
		log.Debugf("got timeout waiting for callback")
		return nil, trace.Wrap(trace.Errorf("timeout waiting for callback"))
	}
}

// PingResponse contains data about the Teleport server like supported authentication
// types, server version, etc.
type PingResponse struct {
	// Auth contains the forms of authentication the auth server supports.
	Auth AuthenticationSettings `json:"auth"`
	// ServerVersion is the version of Teleport that is running.
	ServerVersion string `json:"server_version"`
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

// Ping serves two purposes. The first is to validate the HTTP endpoint of a Teleport proxy. This leads
// to better user experience: users get connection errors before being asked for passwords. The second
// is to return the form of authentication that the server supports. This also leads to better user
// experience: users only get prompted for the type of authentication the server supports.
func Ping(proxyAddr string, insecure bool, pool *x509.CertPool, connectorName string) (*PingResponse, error) {
	clt, _, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	endpoint := clt.Endpoint("webapi", "ping")
	if connectorName != "" {
		endpoint = clt.Endpoint("webapi", "ping", connectorName)
	}

	response, err := clt.Get(endpoint, url.Values{})
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

// SSHAgentLogin issues call to web proxy and receives temp certificate
// if credentials are valid
//
// proxyAddr must be specified as host:port
func SSHAgentLogin(proxyAddr, user, password, otpToken string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool, compatibility string) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	re, err := clt.PostJSON(clt.Endpoint("webapi", "ssh", "certs"), CreateSSHCertReq{
		User:          user,
		Password:      password,
		OTPToken:      otpToken,
		PubKey:        pubKey,
		TTL:           ttl,
		Compatibility: compatibility,
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

// SSHAgentU2FLogin requests a U2F sign request (authentication challenge) via the proxy.
// If the credentials are valid, the proxy wiil return a challenge.
// We then call the official u2f-host binary to perform the signing and pass the signature to the proxy.
// If the authentication succeeds, we will get a temporary certificate back
func SSHAgentU2FLogin(proxyAddr, user, password string, pubKey []byte, ttl time.Duration, insecure bool, pool *x509.CertPool, compatibility string) (*auth.SSHLoginResponse, error) {
	clt, _, err := initClient(proxyAddr, insecure, pool)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u2fSignRequest, err := clt.PostJSON(clt.Endpoint("webapi", "u2f", "signrequest"), U2fSignRequestReq{
		User: user,
		Pass: password,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Pass the JSON-encoded data undecoded to the u2f-host binary
	facet := "https://" + strings.ToLower(proxyAddr)
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

	cmd.Start()
	stdin.Write(u2fSignRequest.Bytes())
	stdin.Close()
	fmt.Println("Please press the button on your U2F key")

	// The origin URL is passed back base64-encoded and the keyHandle is passed back as is.
	// A very long proxy hostname or keyHandle can overflow a fixed-size buffer.
	signResponseLen := 500 + len(u2fSignRequest.Bytes()) + len(proxyAddr)*4/3
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

	re, err := clt.PostJSON(clt.Endpoint("webapi", "u2f", "certs"), CreateSSHCertWithU2FReq{
		User:            user,
		U2FSignResponse: *u2fSignResponse,
		PubKey:          pubKey,
		TTL:             ttl,
		Compatibility:   compatibility,
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

// initClient creates and initializes HTTPS client for talking to teleport proxy HTTPS
// endpoint.
func initClient(proxyAddr string, insecure bool, pool *x509.CertPool) (*WebClient, *url.URL, error) {
	log.Debugf("HTTPS client init(insecure=%v)", insecure)

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
		// skip https cert verification, oh no!
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
