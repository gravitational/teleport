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

// Package webclient provides a client for the Teleport Proxy API endpoints.
package webclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.Do(req)
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.Do(req)
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

// GetTunnelAddr returns the tunnel address either set in an environment variable or retrieved from the web proxy.
func GetTunnelAddr(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool) (string, error) {
	// If TELEPORT_TUNNEL_PUBLIC_ADDR is set, nothing else has to be done, return it.
	if tunnelAddr := os.Getenv(defaults.TunnelPublicAddrEnvar); tunnelAddr != "" {
		return extractHostPort(tunnelAddr)
	}

	// Ping web proxy to retrieve tunnel proxy address.
	pr, err := Find(ctx, proxyAddr, insecure, nil)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return tunnelAddr(proxyAddr, pr.Proxy.SSH)
}

func GetMOTD(ctx context.Context, proxyAddr string, insecure bool, pool *x509.CertPool) (*MotD, error) {
	clt := newWebClient(insecure, pool)
	defer clt.CloseIdleConnections()

	endpoint := fmt.Sprintf("https://%s/webapi/motd", proxyAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("failed to fetch message of the day: %d", resp.StatusCode)
	}

	motd := &MotD{}
	if err := json.NewDecoder(resp.Body).Decode(motd); err != nil {
		return nil, trace.Wrap(err)
	}

	return motd, nil
}

// MotD holds data about the current message of the day.
type MotD struct {
	Text string
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
	// PostgresPublicAddr is advertised to Postgres clients.
	PostgresPublicAddr string `json:"postgres_public_addr,omitempty"`
	// MySQLListenAddr is MySQL proxy listen address.
	MySQLListenAddr string `json:"mysql_listen_addr,omitempty"`
	// MySQLPublicAddr is advertised to MySQL clients.
	MySQLPublicAddr string `json:"mysql_public_addr,omitempty"`
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

	// HasMessageOfTheDay is a flag indicating that the cluster has MOTD
	// banner text that must be retrieved, displayed and acknowledged by
	// the user.
	HasMessageOfTheDay bool `json:"has_motd"`
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

// The tunnel addr is retrieved in the following preference order:
//  1. Reverse Tunnel Public Address.
//  2. SSH Proxy Public Address Host + Tunnel Port.
//  3. HTTP Proxy Public Address Host + Tunnel Port.
//  4. Proxy Address Host + Tunnel Port.
func tunnelAddr(proxyAddr string, settings SSHProxySettings) (string, error) {
	// If a tunnel public address is set, nothing else has to be done, return it.
	if settings.TunnelPublicAddr != "" {
		return extractHostPort(settings.TunnelPublicAddr)
	}

	// Extract the port the tunnel server is listening on.
	tunnelPort := strconv.Itoa(defaults.SSHProxyTunnelListenPort)
	if settings.TunnelListenAddr != "" {
		if port, err := extractPort(settings.TunnelListenAddr); err == nil {
			tunnelPort = port
		}
	}

	// If a tunnel public address has not been set, but a related HTTP or SSH
	// public address has been set, extract the hostname but use the port from
	// the tunnel listen address.
	if settings.SSHPublicAddr != "" {
		if host, err := extractHost(settings.SSHPublicAddr); err == nil {
			return net.JoinHostPort(host, tunnelPort), nil
		}
	}
	if settings.PublicAddr != "" {
		if host, err := extractHost(settings.PublicAddr); err == nil {
			return net.JoinHostPort(host, tunnelPort), nil
		}
	}

	// If nothing is set, fallback to the address dialed with tunnel port.
	host, err := extractHost(proxyAddr)
	if err != nil {
		return "", trace.Wrap(err, "failed to parse the given proxy address")
	}
	return net.JoinHostPort(host, tunnelPort), nil
}

// extractHostPort takes addresses like "tcp://host:port/path" and returns "host:port".
func extractHostPort(addr string) (string, error) {
	if addr == "" {
		return "", trace.BadParameter("missing parameter address")
	}
	if !strings.Contains(addr, "://") {
		addr = "tcp://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", trace.BadParameter("failed to parse %q: %v", addr, err)
	}
	switch u.Scheme {
	case "tcp", "http", "https":
		return u.Host, nil
	default:
		return "", trace.BadParameter("'%v': unsupported scheme: '%v'", addr, u.Scheme)
	}
}

// extractHost takes addresses like "tcp://host:port/path" and returns "host".
func extractHost(addr string) (ra string, err error) {
	parsed, err := extractHostPort(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	host, _, err := net.SplitHostPort(parsed)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			return addr, nil
		}
		return "", trace.Wrap(err)
	}
	return host, nil
}

// extractPort takes addresses like "tcp://host:port/path" and returns "port".
func extractPort(addr string) (string, error) {
	parsed, err := extractHostPort(addr)
	if err != nil {
		return "", trace.Wrap(err)
	}
	_, port, err := net.SplitHostPort(parsed)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return port, nil
}
