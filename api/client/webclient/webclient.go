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
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/observability/tracing"
	tracehttp "github.com/gravitational/teleport/api/observability/tracing/http"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
)

// Config specifies information when building requests with the
// webclient.
type Config struct {
	// Context is a context for creating webclient requests.
	Context context.Context
	// ProxyAddr specifies the teleport proxy address for requests.
	ProxyAddr string
	// Insecure turns off TLS certificate verification when enabled.
	Insecure bool
	// Pool defines the set of root CAs to use when verifying server
	// certificates.
	Pool *x509.CertPool
	// ConnectorName is the name of the ODIC or SAML connector.
	ConnectorName string
	// ExtraHeaders is a map of extra HTTP headers to be included in
	// requests.
	ExtraHeaders map[string]string
	// Timeout is a timeout for requests.
	Timeout time.Duration
	// TraceProvider is used to retrieve a Tracer for creating spans
	TraceProvider oteltrace.TracerProvider
}

// CheckAndSetDefaults checks and sets defaults
func (c *Config) CheckAndSetDefaults() error {
	message := "webclient config: %s"
	if c.Context == nil {
		return trace.BadParameter(message, "missing parameter Context")
	}
	if c.ProxyAddr == "" && os.Getenv(defaults.TunnelPublicAddrEnvar) == "" {
		return trace.BadParameter(message, "missing parameter ProxyAddr")
	}
	if c.Timeout == 0 {
		c.Timeout = defaults.DefaultIOTimeout
	}
	if c.TraceProvider == nil {
		c.TraceProvider = tracing.DefaultProvider()
	}
	return nil
}

// newWebClient creates a new client to the Proxy Web API.
func newWebClient(cfg *Config) (*http.Client, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	rt := utils.NewHTTPRoundTripper(&http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
			RootCAs:            cfg.Pool,
		},
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		IdleConnTimeout: defaults.DefaultIOTimeout,
	}, nil)

	return &http.Client{
		Transport: tracehttp.NewTransport(rt),
		Timeout:   cfg.Timeout,
	}, nil
}

// doWithFallback attempts to execute an HTTP request using https, and then
// fall back to plain HTTP under certain, very specific circumstances.
//   - The caller must specifically allow it via the allowPlainHTTP parameter, and
//   - The target host must resolve to the loopback address.
//
// If these conditions are not met, then the plain-HTTP fallback is not allowed,
// and a the HTTPS failure will be considered final.
func doWithFallback(clt *http.Client, allowPlainHTTP bool, extraHeaders map[string]string, req *http.Request) (*http.Response, error) {
	span := oteltrace.SpanFromContext(req.Context())

	// first try https and see how that goes
	req.URL.Scheme = "https"
	for k, v := range extraHeaders {
		req.Header.Add(k, v)
	}

	log.Debugf("Attempting %s %s%s", req.Method, req.URL.Host, req.URL.Path)
	span.AddEvent("sending https request")
	resp, err := clt.Do(req)

	// If the HTTPS succeeds, return that.
	if err == nil {
		return resp, nil
	}

	// If we're not allowed to try plain HTTP, bail out with whatever error we have.
	// Note that we're only allowed to try plain HTTP on the loopback address, even
	// if the caller says its OK
	if !(allowPlainHTTP && utils.IsLoopback(req.URL.Host)) {
		return nil, trace.Wrap(err)
	}

	// If we get to here a) the HTTPS attempt failed, and b) we're allowed to try
	// clear-text HTTP to see if that works.
	req.URL.Scheme = "http"
	log.Warnf("Request for %s %s%s falling back to PLAIN HTTP", req.Method, req.URL.Host, req.URL.Path)
	span.AddEvent("falling back to http request")
	resp, err = clt.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// Find fetches discovery data by connecting to the given web proxy address.
// It is designed to fetch proxy public addresses without any inefficiencies.
func Find(cfg *Config) (*PingResponse, error) {
	clt, err := newWebClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.CloseIdleConnections()

	ctx, span := cfg.TraceProvider.Tracer("webclient").Start(cfg.Context, "webclient/Find")
	defer span.End()

	endpoint := fmt.Sprintf("https://%s/webapi/find", cfg.ProxyAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := doWithFallback(clt, cfg.Insecure, cfg.ExtraHeaders, req)
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
func Ping(cfg *Config) (*PingResponse, error) {
	clt, err := newWebClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.CloseIdleConnections()

	ctx, span := cfg.TraceProvider.Tracer("webclient").Start(cfg.Context, "webclient/Ping")
	defer span.End()

	endpoint := fmt.Sprintf("https://%s/webapi/ping", cfg.ProxyAddr)
	if cfg.ConnectorName != "" {
		endpoint = fmt.Sprintf("%s/%s", endpoint, cfg.ConnectorName)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := doWithFallback(clt, cfg.Insecure, cfg.ExtraHeaders, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest {
		per := &PingErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(per); err != nil {
			return nil, trace.Wrap(err)
		}
		return nil, errors.New(per.Error.Message)
	}
	pr := &PingResponse{}
	if err := json.NewDecoder(resp.Body).Decode(pr); err != nil {
		return nil, trace.Wrap(err, "cannot parse server response; is %q a Teleport server?", "https://"+cfg.ProxyAddr)
	}

	return pr, nil
}

func GetMOTD(cfg *Config) (*MotD, error) {
	clt, err := newWebClient(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.CloseIdleConnections()

	ctx, span := cfg.TraceProvider.Tracer("webclient").Start(cfg.Context, "webclient/GetMOTD")
	defer span.End()

	endpoint := fmt.Sprintf("https://%s/webapi/motd", cfg.ProxyAddr)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := doWithFallback(clt, cfg.Insecure, cfg.ExtraHeaders, req)
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
	// ClusterName contains the name of the Teleport cluster.
	ClusterName string `json:"cluster_name"`

	// reserved: license_warnings ([]string)
	// AutomaticUpgrades describes whether agents should automatically upgrade.
	AutomaticUpgrades bool `json:"automatic_upgrades"`
}

// PingErrorResponse contains the error message if the requested connector
// does not match one that has been registered.
type PingErrorResponse struct {
	Error PingError `json:"error"`
}

// PingError contains the string message from the PingErrorResponse
type PingError struct {
	Message string `json:"message"`
}

// ProxySettings contains basic information about proxy settings
type ProxySettings struct {
	// Kube is a kubernetes specific proxy section
	Kube KubeProxySettings `json:"kube"`
	// SSH is SSH specific proxy settings
	SSH SSHProxySettings `json:"ssh"`
	// DB contains database access specific proxy settings
	DB DBProxySettings `json:"db"`
	// TLSRoutingEnabled indicates that proxy supports ALPN SNI server where
	// all proxy services are exposed on a single TLS listener (Proxy Web Listener).
	TLSRoutingEnabled bool `json:"tls_routing_enabled"`
	// AssistEnabled is true when Teleport Assist is enabled.
	AssistEnabled bool `json:"assist_enabled"`
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

	// WebListenAddr is the address where the proxy web handler is listening.
	WebListenAddr string `json:"web_listen_addr,omitempty"`

	// PublicAddr is the public address of the HTTP proxy.
	PublicAddr string `json:"public_addr,omitempty"`

	// SSHPublicAddr is the public address of the SSH proxy.
	SSHPublicAddr string `json:"ssh_public_addr,omitempty"`

	// TunnelPublicAddr is the public address of the SSH reverse tunnel.
	TunnelPublicAddr string `json:"ssh_tunnel_public_addr,omitempty"`
}

// DBProxySettings contains database access specific proxy settings.
type DBProxySettings struct {
	// PostgresListenAddr is Postgres proxy listen address.
	PostgresListenAddr string `json:"postgres_listen_addr,omitempty"`
	// PostgresPublicAddr is advertised to Postgres clients.
	PostgresPublicAddr string `json:"postgres_public_addr,omitempty"`
	// MySQLListenAddr is MySQL proxy listen address.
	MySQLListenAddr string `json:"mysql_listen_addr,omitempty"`
	// MySQLPublicAddr is advertised to MySQL clients.
	MySQLPublicAddr string `json:"mysql_public_addr,omitempty"`
	// MongoListenAddr is Mongo proxy listen address.
	MongoListenAddr string `json:"mongo_listen_addr,omitempty"`
	// MongoPublicAddr is advertised to Mongo clients.
	MongoPublicAddr string `json:"mongo_public_addr,omitempty"`
}

// AuthenticationSettings contains information about server authentication
// settings.
type AuthenticationSettings struct {
	// Type is the type of authentication, can be either local or oidc.
	Type string `json:"type"`
	// SecondFactor is the type of second factor to use in authentication.
	SecondFactor constants.SecondFactorType `json:"second_factor,omitempty"`
	// PreferredLocalMFA is a server-side hint for clients to pick an MFA method
	// when various options are available.
	// It is empty if there is nothing to suggest.
	PreferredLocalMFA constants.SecondFactorType `json:"preferred_local_mfa,omitempty"`
	// AllowPasswordless is true if passwordless logins are allowed.
	AllowPasswordless bool `json:"allow_passwordless,omitempty"`
	// AllowHeadless is true if headless logins are allowed.
	AllowHeadless bool `json:"allow_headless,omitempty"`
	// Local contains settings for local authentication.
	Local *LocalSettings `json:"local,omitempty"`
	// Webauthn contains MFA settings for Web Authentication.
	Webauthn *Webauthn `json:"webauthn,omitempty"`
	// U2F contains the Universal Second Factor settings needed for authentication.
	U2F *U2FSettings `json:"u2f,omitempty"`
	// OIDC contains OIDC connector settings needed for authentication.
	OIDC *OIDCSettings `json:"oidc,omitempty"`
	// SAML contains SAML connector settings needed for authentication.
	SAML *SAMLSettings `json:"saml,omitempty"`
	// Github contains Github connector settings needed for authentication.
	Github *GithubSettings `json:"github,omitempty"`
	// PrivateKeyPolicy contains the cluster-wide private key policy.
	PrivateKeyPolicy keys.PrivateKeyPolicy `json:"private_key_policy"`
	// PIVSlot specifies a specific PIV slot to use with hardware key support.
	PIVSlot keys.PIVSlot `json:"piv_slot"`
	// DeviceTrustDisabled provides a clue to Teleport clients on whether to avoid
	// device authentication.
	// Deprecated: Use DeviceTrust.Disabled instead.
	// DELETE IN 16.0, replaced by the DeviceTrust field (codingllama).
	DeviceTrustDisabled bool `json:"device_trust_disabled,omitempty"`
	// DeviceTrust holds cluster-wide device trust settings.
	DeviceTrust DeviceTrustSettings `json:"device_trust,omitempty"`
	// HasMessageOfTheDay is a flag indicating that the cluster has MOTD
	// banner text that must be retrieved, displayed and acknowledged by
	// the user.
	HasMessageOfTheDay bool `json:"has_motd"`
	// LoadAllCAs tells tsh to load CAs for all clusters when trying to ssh into a node.
	LoadAllCAs bool `json:"load_all_cas,omitempty"`
	// DefaultSessionTTL is the TTL requested for user certs if
	// a TTL is not otherwise specified.
	DefaultSessionTTL types.Duration `json:"default_session_ttl"`
}

// LocalSettings holds settings for local authentication.
type LocalSettings struct {
	// Name is the name of the local connector.
	Name string `json:"name"`
}

// Webauthn holds MFA settings for Web Authentication.
type Webauthn struct {
	// RPID is the Webauthn Relying Party ID used by the server.
	RPID string `json:"rp_id"`
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

// DeviceTrustSettings holds cluster-wide device trust settings that are liable
// to change client behavior.
type DeviceTrustSettings struct {
	Disabled   bool `json:"disabled,omitempty"`
	AutoEnroll bool `json:"auto_enroll,omitempty"`
}

func (ps *ProxySettings) TunnelAddr() (string, error) {
	// If TELEPORT_TUNNEL_PUBLIC_ADDR is set, nothing else has to be done, return it.
	if tunnelAddr := os.Getenv(defaults.TunnelPublicAddrEnvar); tunnelAddr != "" {
		addr, err := parseAndJoinHostPort(tunnelAddr)
		return addr, trace.Wrap(err)
	}

	addr, err := ps.tunnelProxyAddr()
	return addr, trace.Wrap(err)
}

// tunnelProxyAddr returns the tunnel proxy address for the proxy settings.
func (ps *ProxySettings) tunnelProxyAddr() (string, error) {
	if ps.TLSRoutingEnabled {
		webPort := ps.getWebPort()
		switch {
		case ps.SSH.PublicAddr != "":
			return parseAndJoinHostPort(ps.SSH.PublicAddr, WithDefaultPort(webPort))
		default:
			return parseAndJoinHostPort(ps.SSH.WebListenAddr, WithDefaultPort(webPort))
		}
	}

	tunnelPort := ps.getTunnelPort()
	switch {
	case ps.SSH.TunnelPublicAddr != "":
		return parseAndJoinHostPort(ps.SSH.TunnelPublicAddr, WithDefaultPort(tunnelPort))
	case ps.SSH.SSHPublicAddr != "":
		return parseAndJoinHostPort(ps.SSH.SSHPublicAddr, WithOverridePort(tunnelPort))
	case ps.SSH.PublicAddr != "":
		return parseAndJoinHostPort(ps.SSH.PublicAddr, WithOverridePort(tunnelPort))
	case ps.SSH.TunnelListenAddr != "":
		return parseAndJoinHostPort(ps.SSH.TunnelListenAddr, WithDefaultPort(tunnelPort))
	default:
		// If nothing else is set, we can at least try the WebListenAddr which should always be set
		return parseAndJoinHostPort(ps.SSH.WebListenAddr, WithDefaultPort(tunnelPort))
	}
}

// SSHProxyHostPort returns the ssh proxy host and port for the proxy settings.
func (ps *ProxySettings) SSHProxyHostPort() (host, port string, err error) {
	if ps.TLSRoutingEnabled {
		webPort := ps.getWebPort()
		switch {
		case ps.SSH.PublicAddr != "":
			return ParseHostPort(ps.SSH.PublicAddr, WithDefaultPort(webPort))
		default:
			return ParseHostPort(ps.SSH.WebListenAddr, WithDefaultPort(webPort))
		}
	}

	sshPort := ps.getSSHPort()
	switch {
	case ps.SSH.SSHPublicAddr != "":
		return ParseHostPort(ps.SSH.SSHPublicAddr, WithDefaultPort(sshPort))
	case ps.SSH.PublicAddr != "":
		return ParseHostPort(ps.SSH.PublicAddr, WithOverridePort(sshPort))
	case ps.SSH.ListenAddr != "":
		return ParseHostPort(ps.SSH.ListenAddr, WithDefaultPort(sshPort))
	default:
		// If nothing else is set, we can at least try the WebListenAddr which should always be set
		return ParseHostPort(ps.SSH.WebListenAddr, WithDefaultPort(sshPort))
	}
}

// getWebPort from WebListenAddr or global default
func (ps *ProxySettings) getWebPort() int {
	if webPort, err := parsePort(ps.SSH.WebListenAddr); err == nil {
		return webPort
	}
	return defaults.StandardHTTPSPort
}

// getSSHPort from ListenAddr or global default
func (ps *ProxySettings) getSSHPort() int {
	if webPort, err := parsePort(ps.SSH.ListenAddr); err == nil {
		return webPort
	}
	return defaults.SSHProxyListenPort
}

// getTunnelPort from TunnelListenAddr or global default
func (ps *ProxySettings) getTunnelPort() int {
	if webPort, err := parsePort(ps.SSH.TunnelListenAddr); err == nil {
		return webPort
	}
	return defaults.SSHProxyTunnelListenPort
}

type ParseHostPortOpt func(host, port string) (hostR, portR string)

// WithDefaultPort replaces the parse port with the default port if empty.
func WithDefaultPort(defaultPort int) ParseHostPortOpt {
	defaultPortString := strconv.Itoa(defaultPort)
	return func(host, port string) (string, string) {
		if port == "" {
			return host, defaultPortString
		}
		return host, port
	}
}

// WithOverridePort replaces the parsed port with the override port.
func WithOverridePort(overridePort int) ParseHostPortOpt {
	overridePortString := strconv.Itoa(overridePort)
	return func(host, port string) (string, string) {
		return host, overridePortString
	}
}

// ParseHostPort parses host and port from the given address.
func ParseHostPort(addr string, opts ...ParseHostPortOpt) (host, port string, err error) {
	if addr == "" {
		return "", "", trace.BadParameter("missing parameter address")
	}
	if !strings.Contains(addr, "://") {
		addr = "tcp://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", "", trace.BadParameter("failed to parse %q: %v", addr, err)
	}
	switch u.Scheme {
	case "tcp", "http", "https":
	default:
		return "", "", trace.BadParameter("'%v': unsupported scheme: '%v'", addr, u.Scheme)
	}
	host, port, err = net.SplitHostPort(u.Host)
	if err != nil && strings.Contains(err.Error(), "missing port in address") {
		host = u.Host
	} else if err != nil {
		return "", "", trace.Wrap(err)
	}
	for _, opt := range opts {
		host, port = opt(host, port)
	}
	return host, port, nil
}

// parseAndJoinHostPort parses host and port from the given address and returns "host:port".
func parseAndJoinHostPort(addr string, opts ...ParseHostPortOpt) (string, error) {
	host, port, err := ParseHostPort(addr, opts...)
	if err != nil {
		return "", trace.Wrap(err)
	} else if port == "" {
		return host, nil
	}
	return net.JoinHostPort(host, port), nil
}

// parsePort parses port from the given address as an integer.
func parsePort(addr string) (int, error) {
	_, port, err := ParseHostPort(addr)
	if err != nil {
		return 0, trace.Wrap(err)
	} else if port == "" {
		return 0, trace.BadParameter("missing port in address %q", addr)
	}
	portI, err := strconv.Atoi(port)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return portI, nil
}
