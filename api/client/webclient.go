/*
Copyright 2020-2021 Gravitational, Inc.

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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/trace"
)

// newWebClient creates a new client to the HTTPS web proxy.
func newWebClient(insecure bool, pool *x509.CertPool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            pool,
				InsecureSkipVerify: insecure,
			},
		},
	}
}

// Find fetches discovery data by connecting to the given web proxy address.
// It is designed to fetch proxy public addresses without any inefficiencies.
func Find(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool) (*PingResponse, error) {
	clt := newWebClient(insecure, pool)
	defer clt.CloseIdleConnections()

	endpoint := fmt.Sprintf("https://%s/webapi/find", proxyAddr)
	resp, err := clt.Get(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer resp.Body.Close()
	pr := &PingResponse{}
	if err := json.NewDecoder(resp.Body).Decode(pr); err != nil {
		return nil, trace.Wrap(err)
	}

	return pr, nil
}

// Ping serves two purposes. The first is to validate the HTTP endpoint of a
// Teleport proxy. This leads to better user experience: users get connection
// errors before being asked for passwords. The second is to return the form
// of authentication that the server supports. This also leads to better user
// experience: users only get prompted for the type of authentication the server supports.
func Ping(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool, connectorName string) (*PingResponse, error) {
	clt := newWebClient(insecure, pool)
	defer clt.CloseIdleConnections()

	endpoint := fmt.Sprintf("https://%s/webapi/ping", proxyAddr)
	if connectorName != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, connectorName)
	}

	resp, err := clt.Get(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	defer resp.Body.Close()
	pr := &PingResponse{}
	if err := json.NewDecoder(resp.Body).Decode(pr); err != nil {
		return nil, trace.Wrap(err)
	}

	return pr, nil
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

	// TunnelListenAddr is the address that the SSH reverse tunnel is
	// listening for connections on.
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
