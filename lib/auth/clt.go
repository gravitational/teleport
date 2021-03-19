/*
Copyright 2015-2021 Gravitational, Inc.

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

package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/u2f"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
	"github.com/jonboulle/clockwork"
)

const (
	// CurrentVersion is a current API version
	CurrentVersion = services.V2

	// MissingNamespaceError is a _very_ common error this file generatets
	MissingNamespaceError = "missing required parameter: namespace"
)

// ContextDialer type alias for backwards compatibility
type ContextDialer = client.ContextDialer

// ContextDialerFunc type alias for backwards compatibility
type ContextDialerFunc = client.ContextDialerFunc

// Client is the Auth API client. It works by connecting to auth servers
// via gRPC and HTTP.
//
// When Teleport servers connect to auth API, they usually establish an SSH
// tunnel first, and then do HTTP-over-SSH. This client is wrapped by auth.TunClient
// in lib/auth/tun.go
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
type Client struct {
	// APIClient is used to make gRPC requests to the server
	*APIClient
	// HTTPClient is used to make http requests to the server
	*HTTPClient
}

// Make sure Client implements all the necessary methods.
var _ ClientI = &Client{}

// NewClient creates a new API client with a connection to a Teleport server.
//
// The client will use the first credentials and the given dialer. If
// no dialer is given, the first address will be used. This address must
// be an auth server address.
//
// NOTE: This client is being deprecated in favor of the gRPC Client in
// teleport/api/client. This Client should only be used internally, or for
// functionality that hasn't been ported to the new client yet.
func NewClient(cfg client.Config, params ...roundtrip.ClientParam) (*Client, error) {
	cfg.DialInBackground = true
	apiClient, err := client.New(context.TODO(), cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// apiClient configures the tls.Config, so we clone it and reuse it for http.
	tlsConfig := apiClient.Config().Clone()
	httpClient, err := NewHTTPClient(cfg, tlsConfig, params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Client{
		APIClient:  apiClient,
		HTTPClient: httpClient,
	}, nil
}

// APIClient is aliased here so that it can be embedded in Client.
type APIClient = client.Client

// HTTPClient is a teleport HTTP API client.
type HTTPClient struct {
	roundtrip.Client
	// transport defines the methods by which the client can reach the server.
	transport *http.Transport
	// TLS holds the TLS config for the http client.
	tls *tls.Config
}

// NewHTTPClient creates a new HTTP client with TLS authentication and the given dialer.
func NewHTTPClient(cfg client.Config, tls *tls.Config, params ...roundtrip.ClientParam) (*HTTPClient, error) {
	dialer := cfg.Dialer
	if dialer == nil {
		if len(cfg.Addrs) == 0 {
			return nil, trace.BadParameter("no addresses to dial")
		}
		contextDialer := client.NewDialer(cfg.KeepAlivePeriod, cfg.DialTimeout)
		dialer = ContextDialerFunc(func(ctx context.Context, network, _ string) (conn net.Conn, err error) {
			for _, addr := range cfg.Addrs {
				conn, err = contextDialer.DialContext(ctx, network, addr)
				if err == nil {
					return conn, nil
				}
			}
			// not wrapping on purpose to preserve the original error
			return nil, err
		})
	}

	// Set the next protocol. This is needed due to the Auth Server using a
	// multiplexer for protocol detection. Unless next protocol is specified
	// it will attempt to upgrade to HTTP2 and at that point there is no way
	// to distinguish between HTTP2/JSON or GPRC.
	tls.NextProtos = []string{teleport.HTTPNextProtoTLS}

	transport := &http.Transport{
		// notice that below roundtrip.Client is passed
		// teleport.APIEndpoint as an address for the API server, this is
		// to make sure client verifies the DNS name of the API server
		// custom DialContext overrides this DNS name to the real address
		// in addition this dialer tries multiple adresses if provided
		DialContext:           dialer.DialContext,
		ResponseHeaderTimeout: defaults.DefaultDialTimeout,
		TLSClientConfig:       tls,

		// Increase the size of the connection pool. This substantially improves the
		// performance of Teleport under load as it reduces the number of TLS
		// handshakes performed.
		MaxIdleConns:        defaults.HTTPMaxIdleConns,
		MaxIdleConnsPerHost: defaults.HTTPMaxIdleConnsPerHost,

		// Limit the total number of connections to the Auth Server. Some hosts allow a low
		// number of connections per process (ulimit) to a host. This is a problem for
		// enhanced session recording auditing which emits so many events to the
		// Audit Log (using the Auth Client) that the connection pool often does not
		// have a free connection to return, so just opens a new one. This quickly
		// leads to hitting the OS limit and the client returning out of file
		// descriptors error.
		MaxConnsPerHost: defaults.HTTPMaxConnsPerHost,

		// IdleConnTimeout defines the maximum amount of time before idle connections
		// are closed. Leaving this unset will lead to connections open forever and
		// will cause memory leaks in a long running process.
		IdleConnTimeout: defaults.HTTPIdleTimeout,
	}

	clientParams := append(
		[]roundtrip.ClientParam{
			roundtrip.HTTPClient(&http.Client{Transport: transport}),
			roundtrip.SanitizerEnabled(true),
		},
		params...,
	)
	httpClient, err := roundtrip.NewClient("https://"+teleport.APIDomain, CurrentVersion, clientParams...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &HTTPClient{
		Client:    *httpClient,
		transport: transport,
		tls:       tls,
	}, nil
}

// Close closes the HTTP client connection to the auth server.
func (c *HTTPClient) Close() {
	c.transport.CloseIdleConnections()
}

// TLSConfig returns the HTTP client's TLS config.
func (c *HTTPClient) TLSConfig() *tls.Config {
	return c.tls
}

// GetTransport returns the HTTP client's transport.
func (c *HTTPClient) GetTransport() *http.Transport {
	return c.transport
}

// ClientConfig contains configuration of the client
// DELETE IN: 7.0.0.
type ClientConfig struct {
	// Addrs is a list of addresses to dial
	Addrs []utils.NetAddr
	// Dialer is a custom dialer that is used instead of Addrs when provided
	Dialer ContextDialer
	// DialTimeout defines how long to attempt dialing before timing out
	DialTimeout time.Duration
	// KeepAlivePeriod defines period between keep alives
	KeepAlivePeriod time.Duration
	// KeepAliveCount specifies the amount of missed keep alives
	// to wait for before declaring the connection as broken
	KeepAliveCount int
	// TLS is the client's TLS config
	TLS *tls.Config
}

// NewTLSClient returns a new TLS client that uses mutual TLS authentication
// and dials the remote server using dialer.
// DELETE IN: 7.0.0.
func NewTLSClient(cfg ClientConfig, params ...roundtrip.ClientParam) (*Client, error) {
	c := client.Config{
		Addrs:           utils.NetAddrsToStrings(cfg.Addrs),
		Dialer:          cfg.Dialer,
		DialTimeout:     cfg.DialTimeout,
		KeepAlivePeriod: cfg.KeepAlivePeriod,
		KeepAliveCount:  cfg.KeepAliveCount,
		Credentials: []client.Credentials{
			client.LoadTLS(cfg.TLS),
		},
	}
	return NewClient(c, params...)
}

// EncodeClusterName encodes cluster name in the SNI hostname
func EncodeClusterName(clusterName string) string {
	// hex is used to hide "." that will prevent wildcard *. entry to match
	return fmt.Sprintf("%v.%v", hex.EncodeToString([]byte(clusterName)), teleport.APIDomain)
}

// DecodeClusterName decodes cluster name, returns NotFound
// if no cluster name is encoded (empty subdomain),
// so servers can detect cases when no server name passed
// returns BadParameter if encoding does not match
func DecodeClusterName(serverName string) (string, error) {
	if serverName == teleport.APIDomain {
		return "", trace.NotFound("no cluster name is encoded")
	}
	const suffix = "." + teleport.APIDomain
	if !strings.HasSuffix(serverName, suffix) {
		return "", trace.NotFound("no cluster name is encoded")
	}
	clusterName := strings.TrimSuffix(serverName, suffix)

	decoded, err := hex.DecodeString(clusterName)
	if err != nil {
		return "", trace.BadParameter("failed to decode cluster name: %v", err)
	}
	return string(decoded), nil
}

// ClientTimeout sets idle and dial timeouts of the HTTP transport
// used by the client.
func ClientTimeout(timeout time.Duration) roundtrip.ClientParam {
	return func(c *roundtrip.Client) error {
		transport, ok := (c.HTTPClient().Transport).(*http.Transport)
		if !ok {
			return nil
		}
		transport.IdleConnTimeout = timeout
		transport.ResponseHeaderTimeout = timeout
		return nil
	}
}

// PostJSON is a generic method that issues http POST request to the server
func (c *Client) PostJSON(endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.PostJSON(context.TODO(), endpoint, val))
}

// PutJSON is a generic method that issues http PUT request to the server
func (c *Client) PutJSON(endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.PutJSON(context.TODO(), endpoint, val))
}

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(endpoint string, vals url.Values, files ...roundtrip.File) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.PostForm(context.TODO(), endpoint, vals, files...))
}

// Get issues http GET request to the server
func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.Get(context.TODO(), u, params))
}

// Delete issues http Delete Request to the server
func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.Delete(context.TODO(), u))
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if successful.
func (c *Client) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("kube", "csr"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re KubeCSRResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return &re, nil
}

// GetSessions returns a list of active sessions in the cluster
// as reported by auth server
func (c *Client) GetSessions(namespace string) ([]session.Session, error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	out, err := c.Get(c.Endpoint("namespaces", namespace, "sessions"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sessions []session.Session
	if err := json.Unmarshal(out.Bytes(), &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetSession returns a session by ID
func (c *Client) GetSession(namespace string, id session.ID) (*session.Session, error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	// saving extra round-trip
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("namespaces", namespace, "sessions", string(id)), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *session.Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

// DeleteSession removes an active session from the backend.
func (c *Client) DeleteSession(namespace string, id session.ID) error {
	if namespace == "" {
		return trace.BadParameter(MissingNamespaceError)
	}
	_, err := c.Delete(c.Endpoint("namespaces", namespace, "sessions", string(id)))
	return trace.Wrap(err)
}

// CreateSession creates new session
func (c *Client) CreateSession(sess session.Session) error {
	if sess.Namespace == "" {
		return trace.BadParameter(MissingNamespaceError)
	}
	_, err := c.PostJSON(c.Endpoint("namespaces", sess.Namespace, "sessions"), createSessionReq{Session: sess})
	return trace.Wrap(err)
}

// UpdateSession updates existing session
func (c *Client) UpdateSession(req session.UpdateRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.PutJSON(c.Endpoint("namespaces", req.Namespace, "sessions", string(req.ID)), updateSessionReq{Update: req})
	return trace.Wrap(err)
}

// GetDomainName returns local auth domain of the current auth server
func (c *Client) GetDomainName() (string, error) {
	out, err := c.Get(c.Endpoint("domain"), url.Values{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	var domain string
	if err := json.Unmarshal(out.Bytes(), &domain); err != nil {
		return "", trace.Wrap(err)
	}
	return domain, nil
}

// GetClusterCACert returns the CAs for the local cluster without signing keys.
func (c *Client) GetClusterCACert() (*LocalCAResponse, error) {
	out, err := c.Get(c.Endpoint("cacert"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var localCA LocalCAResponse
	if err := json.Unmarshal(out.Bytes(), &localCA); err != nil {
		return nil, trace.Wrap(err)
	}
	return &localCA, nil
}

func (c *Client) Close() error {
	c.HTTPClient.Close()
	return c.APIClient.Close()
}

func (c *Client) WaitForDelivery(context.Context) error {
	return nil
}

// CreateCertAuthority not implemented: can only be called locally.
func (c *Client) CreateCertAuthority(ca services.CertAuthority) error {
	return trace.NotImplemented(notImplementedMessage)
}

// RotateCertAuthority starts or restarts certificate authority rotation process.
func (c *Client) RotateCertAuthority(req RotateRequest) error {
	caType := "all"
	if req.Type != "" {
		caType = string(req.Type)
	}
	_, err := c.PostJSON(c.Endpoint("authorities", caType, "rotate"), req)
	return trace.Wrap(err)
}

// RotateExternalCertAuthority rotates external certificate authority,
// this method is used to update only public keys and certificates of the
// the certificate authorities of trusted clusters.
func (c *Client) RotateExternalCertAuthority(ca services.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("authorities", string(ca.GetType()), "rotate", "external"),
		&rotateExternalCertAuthorityRawReq{CA: data})
	return trace.Wrap(err)
}

// UpsertCertAuthority updates or inserts new cert authority
func (c *Client) UpsertCertAuthority(ca services.CertAuthority) error {
	if err := services.ValidateCertAuthority(ca); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("authorities", string(ca.GetType())),
		&upsertCertAuthorityRawReq{CA: data})
	return trace.Wrap(err)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing cert authority
// value matches the value stored in the backend.
func (c *Client) CompareAndSwapCertAuthority(new, existing services.CertAuthority) error {
	return trace.BadParameter("this function is not supported on the client")
}

// GetCertAuthorities returns a list of certificate authorities
func (c *Client) GetCertAuthorities(caType services.CertAuthType, loadKeys bool, opts ...services.MarshalOption) ([]services.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("authorities", string(caType)), url.Values{
		"load_keys": []string{fmt.Sprintf("%t", loadKeys)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, err
	}
	re := make([]services.CertAuthority, len(items))
	for i, raw := range items {
		ca, err := services.UnmarshalCertAuthority(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = ca
	}
	return re, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Client) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool, opts ...services.MarshalOption) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("authorities", string(id.Type), id.DomainName), url.Values{
		"load_keys": []string{fmt.Sprintf("%t", loadSigningKeys)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalCertAuthority(
		out.Bytes(), services.SkipValidation())
}

// DeleteCertAuthority deletes cert authority by ID
func (c *Client) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.Delete(c.Endpoint("authorities", string(id.Type), id.DomainName))
	return trace.Wrap(err)
}

// ActivateCertAuthority not implemented: can only be called locally.
func (c *Client) ActivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeactivateCertAuthority not implemented: can only be called locally.
func (c *Client) DeactivateCertAuthority(id services.CertAuthID) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GenerateToken creates a special provisioning token for a new SSH server
// that is valid for ttl period seconds.
//
// This token is used by SSH server to authenticate with Auth server
// and get signed certificate and private key from the auth server.
//
// If token is not supplied, it will be auto generated and returned.
// If TTL is not supplied, token will be valid until removed.
func (c *Client) GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error) {
	out, err := c.PostJSON(c.Endpoint("tokens"), req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var token string
	if err := json.Unmarshal(out.Bytes(), &token); err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

// RegisterUsingToken calls the auth service API to register a new node using a registration token
// which was previously issued via GenerateToken.
func (c *Client) RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("tokens", "register"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var keys PackedKeys
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, trace.Wrap(err)
	}
	return &keys, nil
}

// RenewCredentials returns a new set of credentials associated
// with the server with the same privileges
func (c *Client) GenerateServerKeys(req GenerateServerKeysRequest) (*PackedKeys, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("server", "credentials"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var keys PackedKeys
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, trace.Wrap(err)
	}

	return &keys, nil
}

// UpsertToken adds provisioning tokens for the auth server
func (c *Client) UpsertToken(ctx context.Context, tok services.ProvisionToken) error {
	_, err := c.PostJSON(c.Endpoint("tokens"), GenerateTokenRequest{
		Token: tok.GetName(),
		Roles: tok.GetRoles(),
		TTL:   backend.TTL(clockwork.NewRealClock(), tok.Expiry()),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTokens returns a list of active invitation tokens for nodes and users
func (c *Client) GetTokens(ctx context.Context, opts ...services.MarshalOption) ([]services.ProvisionToken, error) {
	out, err := c.Get(c.Endpoint("tokens"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tokens []services.ProvisionTokenV1
	if err := json.Unmarshal(out.Bytes(), &tokens); err != nil {
		return nil, trace.Wrap(err)
	}
	return services.ProvisionTokensFromV1(tokens), nil
}

// GetToken returns provisioning token
func (c *Client) GetToken(ctx context.Context, token string) (services.ProvisionToken, error) {
	out, err := c.Get(c.Endpoint("tokens", token), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalProvisionToken(out.Bytes(), services.SkipValidation())
}

// DeleteToken deletes a given provisioning token on the auth server (CA). It
// could be a reset password token or a machine token
func (c *Client) DeleteToken(ctx context.Context, token string) error {
	_, err := c.Delete(c.Endpoint("tokens", token))
	return trace.Wrap(err)
}

// RegisterNewAuthServer is used to register new auth server with token
func (c *Client) RegisterNewAuthServer(ctx context.Context, token string) error {
	_, err := c.PostJSON(c.Endpoint("tokens", "register", "auth"), registerNewAuthServerReq{
		Token: token,
	})
	return trace.Wrap(err)
}

// DELETE IN: 5.1.0
//
// This logic has been moved to KeepAliveServer.
//
// KeepAliveNode updates node keep alive information.
func (c *Client) KeepAliveNode(ctx context.Context, keepAlive services.KeepAlive) error {
	return trace.BadParameter("not implemented, use StreamKeepAlives instead")
}

// KeepAliveServer not implemented: can only be called locally.
func (c *Client) KeepAliveServer(ctx context.Context, keepAlive services.KeepAlive) error {
	return trace.BadParameter("not implemented, use StreamKeepAlives instead")
}

// UpsertNodes bulk inserts nodes.
func (c *Client) UpsertNodes(namespace string, servers []services.Server) error {
	if namespace == "" {
		return trace.BadParameter("missing node namespace")
	}

	bytes, err := services.MarshalServers(servers)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertNodesReq{
		Namespace: namespace,
		Nodes:     bytes,
	}
	_, err = c.PutJSON(c.Endpoint("namespaces", namespace, "nodes"), args)
	return trace.Wrap(err)
}

// DeleteAllNodes deletes all nodes in a given namespace
func (c *Client) DeleteAllNodes(namespace string) error {
	_, err := c.Delete(c.Endpoint("namespaces", namespace, "nodes"))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteNode deletes node in the namespace by name
func (c *Client) DeleteNode(namespace string, name string) error {
	if namespace == "" {
		return trace.BadParameter("missing parameter namespace")
	}
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	_, err := c.Delete(c.Endpoint("namespaces", namespace, "nodes", name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetNodes returns the list of servers registered in the cluster.
func (c *Client) GetNodes(namespace string, opts ...services.MarshalOption) ([]services.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := c.Get(c.Endpoint("namespaces", namespace, "nodes"), url.Values{
		"skip_validation": []string{fmt.Sprintf("%t", cfg.SkipValidation)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]services.Server, len(items))
	for i, raw := range items {
		s, err := services.UnmarshalServer(
			raw,
			services.KindNode,
			services.AddOptions(opts, services.SkipValidation())...)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = s
	}

	return re, nil
}

// UpsertReverseTunnel is used by admins to create a new reverse tunnel
// to the remote proxy to bypass firewall restrictions
func (c *Client) UpsertReverseTunnel(tunnel services.ReverseTunnel) error {
	data, err := services.MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertReverseTunnelRawReq{
		ReverseTunnel: data,
	}
	_, err = c.PostJSON(c.Endpoint("reversetunnels"), args)
	return trace.Wrap(err)
}

// GetReverseTunnel not implemented: can only be called locally.
func (c *Client) GetReverseTunnel(name string, opts ...services.MarshalOption) (services.ReverseTunnel, error) {
	return nil, trace.NotImplemented(notImplementedMessage)
}

// GetReverseTunnels returns the list of created reverse tunnels
func (c *Client) GetReverseTunnels(opts ...services.MarshalOption) ([]services.ReverseTunnel, error) {
	out, err := c.Get(c.Endpoint("reversetunnels"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	tunnels := make([]services.ReverseTunnel, len(items))
	for i, raw := range items {
		tunnel, err := services.UnmarshalReverseTunnel(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels[i] = tunnel
	}
	return tunnels, nil
}

// DeleteReverseTunnel deletes reverse tunnel by domain name
func (c *Client) DeleteReverseTunnel(domainName string) error {
	// this is to avoid confusing error in case if domain empty for example
	// HTTP route will fail producing generic not found error
	// instead we catch the error here
	if strings.TrimSpace(domainName) == "" {
		return trace.BadParameter("empty domain name")
	}
	_, err := c.Delete(c.Endpoint("reversetunnels", domainName))
	return trace.Wrap(err)
}

// UpsertTunnelConnection upserts tunnel connection
func (c *Client) UpsertTunnelConnection(conn services.TunnelConnection) error {
	data, err := services.MarshalTunnelConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertTunnelConnectionRawReq{
		TunnelConnection: data,
	}
	_, err = c.PostJSON(c.Endpoint("tunnelconnections"), args)
	return trace.Wrap(err)
}

// GetTunnelConnections returns tunnel connections for a given cluster
func (c *Client) GetTunnelConnections(clusterName string, opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name parameter")
	}
	out, err := c.Get(c.Endpoint("tunnelconnections", clusterName), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]services.TunnelConnection, len(items))
	for i, raw := range items {
		conn, err := services.UnmarshalTunnelConnection(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}
	return conns, nil
}

// GetAllTunnelConnections returns all tunnel connections
func (c *Client) GetAllTunnelConnections(opts ...services.MarshalOption) ([]services.TunnelConnection, error) {
	out, err := c.Get(c.Endpoint("tunnelconnections"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]services.TunnelConnection, len(items))
	for i, raw := range items {
		conn, err := services.UnmarshalTunnelConnection(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}
	return conns, nil
}

// DeleteTunnelConnection deletes tunnel connection by name
func (c *Client) DeleteTunnelConnection(clusterName string, connName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	if connName == "" {
		return trace.BadParameter("missing parameter connection name")
	}
	_, err := c.Delete(c.Endpoint("tunnelconnections", clusterName, connName))
	return trace.Wrap(err)
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (c *Client) DeleteTunnelConnections(clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	_, err := c.Delete(c.Endpoint("tunnelconnections", clusterName))
	return trace.Wrap(err)
}

// DeleteAllTokens not implemented: can only be called locally.
func (c *Client) DeleteAllTokens() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllTunnelConnections deletes all tunnel connections
func (c *Client) DeleteAllTunnelConnections() error {
	_, err := c.Delete(c.Endpoint("tunnelconnections"))
	return trace.Wrap(err)
}

// AddUserLoginAttempt logs user login attempt
func (c *Client) AddUserLoginAttempt(user string, attempt services.LoginAttempt, ttl time.Duration) error {
	panic("not implemented")
}

// GetUserLoginAttempts returns user login attempts
func (c *Client) GetUserLoginAttempts(user string) ([]services.LoginAttempt, error) {
	panic("not implemented")
}

// GetRemoteClusters returns a list of remote clusters
func (c *Client) GetRemoteClusters(opts ...services.MarshalOption) ([]services.RemoteCluster, error) {
	out, err := c.Get(c.Endpoint("remoteclusters"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	conns := make([]services.RemoteCluster, len(items))
	for i, raw := range items {
		conn, err := services.UnmarshalRemoteCluster(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns[i] = conn
	}
	return conns, nil
}

// GetRemoteCluster returns a remote cluster by name
func (c *Client) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("missing cluster name")
	}
	out, err := c.Get(c.Endpoint("remoteclusters", clusterName), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalRemoteCluster(out.Bytes(), services.SkipValidation())
}

// DeleteRemoteCluster deletes remote cluster by name
func (c *Client) DeleteRemoteCluster(clusterName string) error {
	if clusterName == "" {
		return trace.BadParameter("missing parameter cluster name")
	}
	_, err := c.Delete(c.Endpoint("remoteclusters", clusterName))
	return trace.Wrap(err)
}

// DeleteAllRemoteClusters deletes all remote clusters
func (c *Client) DeleteAllRemoteClusters() error {
	_, err := c.Delete(c.Endpoint("remoteclusters"))
	return trace.Wrap(err)
}

// CreateRemoteCluster creates remote cluster resource
func (c *Client) CreateRemoteCluster(rc services.RemoteCluster) error {
	data, err := services.MarshalRemoteCluster(rc)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &createRemoteClusterRawReq{
		RemoteCluster: data,
	}
	_, err = c.PostJSON(c.Endpoint("remoteclusters"), args)
	return trace.Wrap(err)
}

// UpsertAuthServer is used by auth servers to report their presence
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertAuthServer(s services.Server) error {
	data, err := services.MarshalServer(s)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertServerRawReq{
		Server: data,
	}
	_, err = c.PostJSON(c.Endpoint("authservers"), args)
	return trace.Wrap(err)
}

// GetAuthServers returns the list of auth servers registered in the cluster.
func (c *Client) GetAuthServers() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("authservers"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]services.Server, len(items))
	for i, raw := range items {
		server, err := services.UnmarshalServer(raw, services.KindAuthServer, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
}

// DeleteAllAuthServers not implemented: can only be called locally.
func (c *Client) DeleteAllAuthServers() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAuthServer not implemented: can only be called locally.
func (c *Client) DeleteAuthServer(name string) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertProxy is used by proxies to report their presence
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertProxy(s services.Server) error {
	data, err := services.MarshalServer(s)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertServerRawReq{
		Server: data,
	}
	_, err = c.PostJSON(c.Endpoint("proxies"), args)
	return trace.Wrap(err)
}

// GetProxies returns the list of auth servers registered in the cluster.
func (c *Client) GetProxies() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("proxies"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]services.Server, len(items))
	for i, raw := range items {
		server, err := services.UnmarshalServer(raw, services.KindProxy, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
}

// DeleteAllProxies deletes all proxies
func (c *Client) DeleteAllProxies() error {
	_, err := c.Delete(c.Endpoint("proxies"))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteProxy deletes proxy by name
func (c *Client) DeleteProxy(name string) error {
	if name == "" {
		return trace.BadParameter("missing parameter name")
	}
	_, err := c.Delete(c.Endpoint("proxies", name))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetU2FAppID returns U2F settings, like App ID and Facets
func (c *Client) GetU2FAppID() (string, error) {
	out, err := c.Get(c.Endpoint("u2f", "appID"), url.Values{})
	if err != nil {
		return "", trace.Wrap(err)
	}
	var appid string
	if err := json.Unmarshal(out.Bytes(), &appid); err != nil {
		return "", trace.Wrap(err)
	}
	return appid, nil
}

// UpsertPassword updates web access password for the user
func (c *Client) UpsertPassword(user string, password []byte) error {
	_, err := c.PostJSON(
		c.Endpoint("users", user, "web", "password"),
		upsertPasswordReq{
			Password: string(password),
		})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpsertUser user updates user entry.
func (c *Client) UpsertUser(user services.User) error {
	data, err := services.MarshalUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("users"), &upsertUserRawReq{User: data})
	return trace.Wrap(err)
}

// ChangePassword updates users password based on the old password.
func (c *Client) ChangePassword(req services.ChangePasswordReq) error {
	_, err := c.PutJSON(c.Endpoint("users", req.User, "web", "password"), req)
	return trace.Wrap(err)
}

// CheckPassword checks if the suplied web access password is valid.
func (c *Client) CheckPassword(user string, password []byte, otpToken string) error {
	_, err := c.PostJSON(
		c.Endpoint("users", user, "web", "password", "check"),
		checkPasswordReq{
			Password: string(password),
			OTPToken: otpToken,
		})
	return trace.Wrap(err)
}

// GetMFAAuthenticateChallenge generates request for user trying to authenticate with U2F token
func (c *Client) GetMFAAuthenticateChallenge(user string, password []byte) (*MFAAuthenticateChallenge, error) {
	out, err := c.PostJSON(
		c.Endpoint("u2f", "users", user, "sign"),
		signInReq{
			Password: string(password),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var signRequest *MFAAuthenticateChallenge
	if err := json.Unmarshal(out.Bytes(), &signRequest); err != nil {
		return nil, err
	}
	return signRequest, nil
}

// ExtendWebSession creates a new web session for a user based on another
// valid web session
func (c *Client) ExtendWebSession(user string, prevSessionID string, accessRequestID string) (services.WebSession, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "sessions"),
		createWebSessionReq{
			PrevSessionID:   prevSessionID,
			AccessRequestID: accessRequestID,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalWebSession(out.Bytes())
}

// CreateWebSession creates a new web session for a user
func (c *Client) CreateWebSession(user string) (services.WebSession, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "sessions"),
		createWebSessionReq{},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalWebSession(out.Bytes())
}

// AuthenticateWebUser authenticates web user, creates and  returns web session
// in case if authentication is successful
func (c *Client) AuthenticateWebUser(req AuthenticateUserRequest) (services.WebSession, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", req.Username, "web", "authenticate"),
		req,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalWebSession(out.Bytes())
}

// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
// short lived certificates as a result
func (c *Client) AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", req.Username, "ssh", "authenticate"),
		req,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re SSHLoginResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return &re, nil
}

// GetWebSessionInfo checks if a web sesion is valid, returns session id in case if
// it is valid, or error otherwise.
func (c *Client) GetWebSessionInfo(ctx context.Context, user, sessionID string) (services.WebSession, error) {
	out, err := c.Get(
		c.Endpoint("users", user, "web", "sessions", sessionID), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalWebSession(out.Bytes())
}

// DeleteWebSession deletes the web session specified with sid for the given user
func (c *Client) DeleteWebSession(user string, sid string) error {
	_, err := c.Delete(c.Endpoint("users", user, "web", "sessions", sid))
	return trace.Wrap(err)
}

// GenerateKeyPair generates SSH private/public key pair optionally protected
// by password. If the pass parameter is an empty string, the key pair
// is not password-protected.
func (c *Client) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	out, err := c.PostJSON(c.Endpoint("keypair"), generateKeyPairReq{Password: pass})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var kp *generateKeyPairResponse
	if err := json.Unmarshal(out.Bytes(), &kp); err != nil {
		return nil, nil, err
	}
	return kp.PrivKey, []byte(kp.PubKey), err
}

// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
// plain text format, signs it using Host Certificate Authority private key and returns the
// resulting certificate.
func (c *Client) GenerateHostCert(
	key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {

	out, err := c.PostJSON(c.Endpoint("ca", "host", "certs"),
		generateHostCertReq{
			Key:         key,
			HostID:      hostID,
			NodeName:    nodeName,
			Principals:  principals,
			ClusterName: clusterName,
			Roles:       roles,
			TTL:         ttl,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cert string
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}

	return []byte(cert), nil
}

// GetSignupU2FRegisterRequest generates sign request for user trying to sign up with invite tokenx
func (c *Client) GetSignupU2FRegisterRequest(token string) (*u2f.RegisterChallenge, error) {
	out, err := c.Get(c.Endpoint("u2f", "signuptokens", token), url.Values{})
	if err != nil {
		return nil, err
	}
	var u2fRegReq u2f.RegisterChallenge
	if err := json.Unmarshal(out.Bytes(), &u2fRegReq); err != nil {
		return nil, err
	}
	return &u2fRegReq, nil
}

// ChangePasswordWithToken changes user password with ResetPasswordToken
func (c *Client) ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.WebSession, error) {
	out, err := c.PostJSON(c.Endpoint("web", "password", "token"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalWebSession(out.Bytes())
}

// UpsertOIDCConnector updates or creates OIDC connector
func (c *Client) UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error {
	if err := c.APIClient.UpsertOIDCConnector(ctx, connector); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	data, err := services.MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("oidc", "connectors"), &upsertOIDCConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCConnector returns OIDC connector information by id
func (c *Client) GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (services.OIDCConnector, error) {
	if resp, err := c.APIClient.GetOIDCConnector(ctx, id, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("oidc", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	return services.UnmarshalOIDCConnector(out.Bytes(), services.SkipValidation())
}

// GetOIDCConnectors gets OIDC connectors list
func (c *Client) GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]services.OIDCConnector, error) {
	if resp, err := c.APIClient.GetOIDCConnectors(ctx, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	out, err := c.Get(c.Endpoint("oidc", "connectors"),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.OIDCConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalOIDCConnector(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteOIDCConnector deletes OIDC connector by ID
func (c *Client) DeleteOIDCConnector(ctx context.Context, connectorID string) error {
	if err := c.APIClient.DeleteOIDCConnector(ctx, connectorID); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	if connectorID == "" {
		return trace.BadParameter("missing connector id")
	}
	_, err := c.Delete(c.Endpoint("oidc", "connectors", connectorID))
	return trace.Wrap(err)
}

// CreateOIDCAuthRequest creates OIDCAuthRequest
func (c *Client) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	out, err := c.PostJSON(c.Endpoint("oidc", "requests", "create"), createOIDCAuthRequestReq{
		Req: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response *services.OIDCAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// ValidateOIDCAuthCallback validates OIDC auth callback returned from redirect
func (c *Client) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	out, err := c.PostJSON(c.Endpoint("oidc", "requests", "validate"), validateOIDCAuthCallbackReq{
		Query: q,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rawResponse *oidcAuthRawResponse
	if err := json.Unmarshal(out.Bytes(), &rawResponse); err != nil {
		return nil, trace.Wrap(err)
	}
	response := OIDCAuthResponse{
		Username: rawResponse.Username,
		Identity: rawResponse.Identity,
		Cert:     rawResponse.Cert,
		Req:      rawResponse.Req,
		TLSCert:  rawResponse.TLSCert,
	}
	if len(rawResponse.Session) != 0 {
		session, err := services.UnmarshalWebSession(rawResponse.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	response.HostSigners = make([]services.CertAuthority, len(rawResponse.HostSigners))
	for i, raw := range rawResponse.HostSigners {
		ca, err := services.UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners[i] = ca
	}
	return &response, nil
}

// CreateSAMLConnector creates SAML connector
func (c *Client) CreateSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	data, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("saml", "connectors"), &createSAMLConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertSAMLConnector updates or creates SAML connector
func (c *Client) UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error {
	if err := c.APIClient.UpsertSAMLConnector(ctx, connector); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	data, err := services.MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("saml", "connectors"), &upsertSAMLConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLConnector returns SAML connector information by id
func (c *Client) GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (services.SAMLConnector, error) {
	if resp, err := c.APIClient.GetSAMLConnector(ctx, id, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("saml", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalSAMLConnector(out.Bytes(), services.SkipValidation())
}

// GetSAMLConnectors gets SAML connectors list
func (c *Client) GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]services.SAMLConnector, error) {
	if resp, err := c.APIClient.GetSAMLConnectors(ctx, withSecrets); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	out, err := c.Get(c.Endpoint("saml", "connectors"),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.SAMLConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalSAMLConnector(raw, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteSAMLConnector deletes SAML connector by ID
func (c *Client) DeleteSAMLConnector(ctx context.Context, connectorID string) error {
	if err := c.APIClient.DeleteSAMLConnector(ctx, connectorID); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 8.0
	if connectorID == "" {
		return trace.BadParameter("missing connector id")
	}
	_, err := c.Delete(c.Endpoint("saml", "connectors", connectorID))
	return trace.Wrap(err)
}

// CreateSAMLAuthRequest creates SAML AuthnRequest
func (c *Client) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	out, err := c.PostJSON(c.Endpoint("saml", "requests", "create"), createSAMLAuthRequestReq{
		Req: req,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response *services.SAMLAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// ValidateSAMLResponse validates response returned by SAML identity provider
func (c *Client) ValidateSAMLResponse(re string) (*SAMLAuthResponse, error) {
	out, err := c.PostJSON(c.Endpoint("saml", "requests", "validate"), validateSAMLResponseReq{
		Response: re,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rawResponse *samlAuthRawResponse
	if err := json.Unmarshal(out.Bytes(), &rawResponse); err != nil {
		return nil, trace.Wrap(err)
	}
	response := SAMLAuthResponse{
		Username: rawResponse.Username,
		Identity: rawResponse.Identity,
		Cert:     rawResponse.Cert,
		Req:      rawResponse.Req,
		TLSCert:  rawResponse.TLSCert,
	}
	if len(rawResponse.Session) != 0 {
		session, err := services.UnmarshalWebSession(rawResponse.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	response.HostSigners = make([]services.CertAuthority, len(rawResponse.HostSigners))
	for i, raw := range rawResponse.HostSigners {
		ca, err := services.UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners[i] = ca
	}
	return &response, nil
}

// CreateGithubConnector creates a new Github connector
func (c *Client) CreateGithubConnector(connector services.GithubConnector) error {
	bytes, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("github", "connectors"), &createGithubConnectorRawReq{
		Connector: bytes,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertGithubConnector creates or updates a Github connector
func (c *Client) UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error {
	bytes, err := services.MarshalGithubConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("github", "connectors"), &upsertGithubConnectorRawReq{
		Connector: bytes,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetGithubConnectors returns all configured Github connectors
func (c *Client) GetGithubConnectors(ctx context.Context, withSecrets bool) ([]services.GithubConnector, error) {
	out, err := c.Get(c.Endpoint("github", "connectors"), url.Values{
		"with_secrets": []string{strconv.FormatBool(withSecrets)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]services.GithubConnector, len(items))
	for i, raw := range items {
		connector, err := services.UnmarshalGithubConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// GetGithubConnector returns the specified Github connector
func (c *Client) GetGithubConnector(ctx context.Context, id string, withSecrets bool) (services.GithubConnector, error) {
	out, err := c.Get(c.Endpoint("github", "connectors", id), url.Values{
		"with_secrets": []string{strconv.FormatBool(withSecrets)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalGithubConnector(out.Bytes())
}

// DeleteGithubConnector deletes the specified Github connector
func (c *Client) DeleteGithubConnector(ctx context.Context, id string) error {
	_, err := c.Delete(c.Endpoint("github", "connectors", id))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
func (c *Client) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	out, err := c.PostJSON(c.Endpoint("github", "requests", "create"),
		createGithubAuthRequestReq{Req: req})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response services.GithubAuthRequest
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return &response, nil
}

// ValidateGithubAuthCallback validates Github auth callback returned from redirect
func (c *Client) ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error) {
	out, err := c.PostJSON(c.Endpoint("github", "requests", "validate"),
		validateGithubAuthCallbackReq{Query: q})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rawResponse githubAuthRawResponse
	if err := json.Unmarshal(out.Bytes(), &rawResponse); err != nil {
		return nil, trace.Wrap(err)
	}
	response := GithubAuthResponse{
		Username: rawResponse.Username,
		Identity: rawResponse.Identity,
		Cert:     rawResponse.Cert,
		Req:      rawResponse.Req,
		TLSCert:  rawResponse.TLSCert,
	}
	if len(rawResponse.Session) != 0 {
		session, err := services.UnmarshalWebSession(
			rawResponse.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	response.HostSigners = make([]services.CertAuthority, len(rawResponse.HostSigners))
	for i, raw := range rawResponse.HostSigners {
		ca, err := services.UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners[i] = ca
	}
	return &response, nil
}

// EmitAuditEventLegacy sends an auditable event to the auth server (part of events.IAuditLog interface)
func (c *Client) EmitAuditEventLegacy(event events.Event, fields events.EventFields) error {
	_, err := c.PostJSON(c.Endpoint("events"), &auditEventReq{
		Event:  event,
		Fields: fields,
		// Send "type" as well for backwards compatibility.
		Type: event.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostSessionSlice allows clients to submit session stream chunks to the audit log
// (part of evets.IAuditLog interface)
//
// The data is POSTed to HTTP server as a simple binary body (no encodings of any
// kind are needed)
func (c *Client) PostSessionSlice(slice events.SessionSlice) error {
	data, err := slice.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}
	r, err := http.NewRequest("POST", c.Endpoint("namespaces", slice.Namespace, "sessions", slice.SessionID, "slice"), bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	r.Header.Set("Content-Type", "application/grpc")
	c.Client.SetAuthHeader(r.Header)
	re, err := c.Client.HTTPClient().Do(r)
	if err != nil {
		return trace.Wrap(err)
	}
	// we **must** consume response by reading all of its body, otherwise the http
	// client will allocate a new connection for subsequent requests
	defer re.Body.Close()
	responseBytes, _ := ioutil.ReadAll(re.Body)
	return trace.ReadError(re.StatusCode, responseBytes)
}

// GetSessionChunk allows clients to receive a byte array (chunk) from a recorded
// session stream, starting from 'offset', up to 'max' in length. The upper bound
// of 'max' is set to events.MaxChunkBytes
func (c *Client) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	response, err := c.Get(c.Endpoint("namespaces", namespace, "sessions", string(sid), "stream"), url.Values{
		"offset": []string{strconv.Itoa(offsetBytes)},
		"bytes":  []string{strconv.Itoa(maxBytes)},
	})
	if err != nil {
		log.Error(err)
		return nil, trace.Wrap(err)
	}
	return response.Bytes(), nil
}

// UploadSessionRecording uploads session recording to the audit server
func (c *Client) UploadSessionRecording(r events.SessionRecording) error {
	file := roundtrip.File{
		Name:     "recording",
		Filename: "recording",
		Reader:   r.Recording,
	}
	values := url.Values{
		"sid":       []string{string(r.SessionID)},
		"namespace": []string{r.Namespace},
	}
	_, err := c.PostForm(c.Endpoint("namespaces", r.Namespace, "sessions", string(r.SessionID), "recording"), values, file)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Returns events that happen during a session sorted by time
// (oldest first).
//
// afterN allows to filter by "newer than N" value where N is the cursor ID
// of previously returned bunch (good for polling for latest)
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (c *Client) GetSessionEvents(namespace string, sid session.ID, afterN int, includePrintEvents bool) (retval []events.EventFields, err error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	query := make(url.Values)
	if afterN > 0 {
		query.Set("after", strconv.Itoa(afterN))
	}
	if includePrintEvents {
		query.Set("print", fmt.Sprintf("%v", includePrintEvents))
	}
	response, err := c.Get(c.Endpoint("namespaces", namespace, "sessions", string(sid), "events"), query)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retval = make([]events.EventFields, 0)
	if err := json.Unmarshal(response.Bytes(), &retval); err != nil {
		return nil, trace.Wrap(err)
	}
	return retval, nil
}

// SearchEvents returns events that fit the criteria
func (c *Client) SearchEvents(from, to time.Time, query string, limit int) ([]events.EventFields, error) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, trace.BadParameter("query")
	}
	q.Set("from", from.Format(time.RFC3339))
	q.Set("to", to.Format(time.RFC3339))
	q.Set("limit", fmt.Sprintf("%v", limit))
	response, err := c.Get(c.Endpoint("events"), q)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	retval := make([]events.EventFields, 0)
	if err := json.Unmarshal(response.Bytes(), &retval); err != nil {
		return nil, trace.Wrap(err)
	}
	return retval, nil
}

// SearchSessionEvents returns session related events to find completed sessions.
func (c *Client) SearchSessionEvents(from, to time.Time, limit int) ([]events.EventFields, error) {
	query := url.Values{
		"to":    []string{to.Format(time.RFC3339)},
		"from":  []string{from.Format(time.RFC3339)},
		"limit": []string{fmt.Sprintf("%v", limit)},
	}

	response, err := c.Get(c.Endpoint("events", "session"), query)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	retval := make([]events.EventFields, 0)
	if err := json.Unmarshal(response.Bytes(), &retval); err != nil {
		return nil, trace.Wrap(err)
	}

	return retval, nil
}

// GetNamespaces returns a list of namespaces
func (c *Client) GetNamespaces() ([]services.Namespace, error) {
	out, err := c.Get(c.Endpoint("namespaces"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []services.Namespace
	if err := utils.FastUnmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return re, nil
}

// GetNamespace returns namespace by name
func (c *Client) GetNamespace(name string) (*services.Namespace, error) {
	if name == "" {
		return nil, trace.BadParameter("missing namespace name")
	}
	out, err := c.Get(c.Endpoint("namespaces", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalNamespace(out.Bytes(), services.SkipValidation())
}

// UpsertNamespace upserts namespace
func (c *Client) UpsertNamespace(ns services.Namespace) error {
	_, err := c.PostJSON(c.Endpoint("namespaces"), upsertNamespaceReq{Namespace: ns})
	return trace.Wrap(err)
}

// DeleteNamespace deletes namespace by name
func (c *Client) DeleteNamespace(name string) error {
	_, err := c.Delete(c.Endpoint("namespaces", name))
	return trace.Wrap(err)
}

// GetRoles returns a list of roles
func (c *Client) GetRoles(ctx context.Context) ([]services.Role, error) {
	if resp, err := c.APIClient.GetRoles(ctx); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 7.0
	out, err := c.Get(c.Endpoint("roles"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	roles := make([]services.Role, len(items))
	for i, roleBytes := range items {
		role, err := services.UnmarshalRole(roleBytes, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles[i] = role
	}
	return roles, nil
}

// CreateRole not implemented: can only be called locally.
func (c *Client) CreateRole(role services.Role) error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertRole creates or updates role
func (c *Client) UpsertRole(ctx context.Context, role services.Role) error {
	if err := c.APIClient.UpsertRole(ctx, role); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 7.0
	data, err := services.MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("roles"), &upsertRoleRawReq{Role: data})
	return trace.Wrap(err)
}

// GetRole returns role by name
func (c *Client) GetRole(ctx context.Context, name string) (services.Role, error) {
	if resp, err := c.APIClient.GetRole(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return nil, trace.Wrap(err)
		}
	} else {
		return resp, nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 7.0
	if name == "" {
		return nil, trace.BadParameter("missing name")
	}
	out, err := c.Get(c.Endpoint("roles", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := services.UnmarshalRole(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// DeleteRole deletes role by name
func (c *Client) DeleteRole(ctx context.Context, name string) error {
	if err := c.APIClient.DeleteRole(ctx, name); err != nil {
		if !trace.IsNotImplemented(err) {
			return trace.Wrap(err)
		}
	} else {
		return nil
	}

	// fallback to http if grpc is not implemented
	// DELETE IN 7.0
	if name == "" {
		return trace.BadParameter("missing name")
	}
	_, err := c.Delete(c.Endpoint("roles", name))
	return trace.Wrap(err)
}

// GetClusterConfig returns cluster level configuration information.
func (c *Client) GetClusterConfig(opts ...services.MarshalOption) (services.ClusterConfig, error) {
	out, err := c.Get(c.Endpoint("configuration"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := services.UnmarshalClusterConfig(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cc, err
}

// SetClusterConfig sets cluster level configuration information.
func (c *Client) SetClusterConfig(cc services.ClusterConfig) error {
	data, err := services.MarshalClusterConfig(cc)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("configuration"), &setClusterConfigReq{ClusterConfig: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetClusterName returns a cluster name
func (c *Client) GetClusterName(opts ...services.MarshalOption) (services.ClusterName, error) {
	out, err := c.Get(c.Endpoint("configuration", "name"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := services.UnmarshalClusterName(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cn, err
}

// SetClusterName sets cluster name once, will
// return Already Exists error if the name is already set
func (c *Client) SetClusterName(cn services.ClusterName) error {
	data, err := services.MarshalClusterName(cn)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("configuration", "name"), &setClusterNameReq{ClusterName: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpsertClusterName not implemented: can only be called locally.
func (c *Client) UpsertClusterName(cn services.ClusterName) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteStaticTokens deletes static tokens
func (c *Client) DeleteStaticTokens() error {
	_, err := c.Delete(c.Endpoint("configuration", "static_tokens"))
	return trace.Wrap(err)
}

// GetStaticTokens returns a list of static register tokens
func (c *Client) GetStaticTokens() (services.StaticTokens, error) {
	out, err := c.Get(c.Endpoint("configuration", "static_tokens"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	st, err := services.UnmarshalStaticTokens(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return st, err
}

// SetStaticTokens sets a list of static register tokens
func (c *Client) SetStaticTokens(st services.StaticTokens) error {
	data, err := services.MarshalStaticTokens(st)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("configuration", "static_tokens"), &setStaticTokensReq{StaticTokens: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Client) GetAuthPreference() (services.AuthPreference, error) {
	out, err := c.Get(c.Endpoint("authentication", "preference"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cap, err := services.UnmarshalAuthPreference(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cap, nil
}

func (c *Client) SetAuthPreference(cap services.AuthPreference) error {
	data, err := services.MarshalAuthPreference(cap)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("authentication", "preference"), &setClusterAuthPreferenceReq{ClusterAuthPreference: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteAuthPreference not implemented: can only be called locally.
func (c *Client) DeleteAuthPreference(context.Context) error {
	return trace.NotImplemented(notImplementedMessage)
}

// GetLocalClusterName returns local cluster name
func (c *Client) GetLocalClusterName() (string, error) {
	return c.GetDomainName()
}

// DeleteClusterConfig not implemented: can only be called locally.
func (c *Client) DeleteClusterConfig() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteClusterName not implemented: can only be called locally.
func (c *Client) DeleteClusterName() error {
	return trace.NotImplemented(notImplementedMessage)
}

// UpsertLocalClusterName not implemented: can only be called locally.
func (c *Client) UpsertLocalClusterName(string) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertAuthorities not implemented: can only be called locally.
func (c *Client) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllReverseTunnels not implemented: can only be called locally.
func (c *Client) DeleteAllReverseTunnels() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllCertNamespaces not implemented: can only be called locally.
func (c *Client) DeleteAllNamespaces() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllRoles not implemented: can only be called locally.
func (c *Client) DeleteAllRoles() error {
	return trace.NotImplemented(notImplementedMessage)
}

// DeleteAllUsers not implemented: can only be called locally.
func (c *Client) DeleteAllUsers() error {
	return trace.NotImplemented(notImplementedMessage)
}

func (c *Client) GetTrustedCluster(ctx context.Context, name string) (services.TrustedCluster, error) {
	out, err := c.Get(c.Endpoint("trustedclusters", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedCluster, err := services.UnmarshalTrustedCluster(out.Bytes(), services.SkipValidation())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedCluster, nil
}

func (c *Client) GetTrustedClusters(ctx context.Context) ([]services.TrustedCluster, error) {
	out, err := c.Get(c.Endpoint("trustedclusters"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusters := make([]services.TrustedCluster, len(items))
	for i, bytes := range items {
		trustedCluster, err := services.UnmarshalTrustedCluster(bytes, services.SkipValidation())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		trustedClusters[i] = trustedCluster
	}

	return trustedClusters, nil
}

// UpsertTrustedCluster creates or updates a trusted cluster.
func (c *Client) UpsertTrustedCluster(ctx context.Context, trustedCluster services.TrustedCluster) (services.TrustedCluster, error) {
	trustedClusterBytes, err := services.MarshalTrustedCluster(trustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("trustedclusters"), &upsertTrustedClusterReq{
		TrustedCluster: trustedClusterBytes,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalTrustedCluster(out.Bytes())
}

func (c *Client) ValidateTrustedCluster(validateRequest *ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error) {
	validateRequestRaw, err := validateRequest.ToRaw()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out, err := c.PostJSON(c.Endpoint("trustedclusters", "validate"), validateRequestRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var validateResponseRaw ValidateTrustedClusterResponseRaw
	err = json.Unmarshal(out.Bytes(), &validateResponseRaw)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	validateResponse, err := validateResponseRaw.ToNative()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return validateResponse, nil
}

// DeleteTrustedCluster deletes a trusted cluster by name.
func (c *Client) DeleteTrustedCluster(ctx context.Context, name string) error {
	_, err := c.Delete(c.Endpoint("trustedclusters", name))
	return trace.Wrap(err)
}

// CreateResetPasswordToken creates reset password token
func (c *Client) CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error) {
	return c.APIClient.CreateResetPasswordToken(ctx, &proto.CreateResetPasswordTokenRequest{
		Name: req.Name,
		TTL:  proto.Duration(req.TTL),
		Type: req.Type,
	})
}

// GetAppServers gets all application servers.
func (c *Client) GetAppServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.Server, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.APIClient.GetAppServers(ctx, namespace, cfg.SkipValidation)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp, nil
}

// GetDatabaseServers returns all registered database proxy servers.
func (c *Client) GetDatabaseServers(ctx context.Context, namespace string, opts ...services.MarshalOption) ([]types.DatabaseServer, error) {
	cfg, err := services.CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := c.APIClient.GetDatabaseServers(ctx, namespace, cfg.SkipValidation)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return resp, nil
}

// UpsertAppSession not implemented: can only be called locally.
func (c *Client) UpsertAppSession(ctx context.Context, session services.WebSession) error {
	return trace.NotImplemented(notImplementedMessage)
}

// ResumeAuditStream resumes existing audit stream.
// This is a wrapper on the grpc endpoint and is deprecated.
// DELETE IN 7.0.0
func (c *Client) ResumeAuditStream(ctx context.Context, sid session.ID, uploadID string) (events.Stream, error) {
	return c.APIClient.ResumeAuditStream(ctx, string(sid), uploadID)
}

// CreateAuditStream creates new audit stream.
// This is a wrapper on the grpc endpoint and is deprecated.
// DELETE IN 7.0.0
func (c *Client) CreateAuditStream(ctx context.Context, sid session.ID) (events.Stream, error) {
	return c.APIClient.CreateAuditStream(ctx, string(sid))
}

// WebService implements features used by Web UI clients
type WebService interface {
	// GetWebSessionInfo checks if a web sesion is valid, returns session id in case if
	// it is valid, or error otherwise.
	GetWebSessionInfo(ctx context.Context, user, sessionID string) (types.WebSession, error)
	// ExtendWebSession creates a new web session for a user based on another
	// valid web session
	ExtendWebSession(user, prevSessionID, accessRequestID string) (types.WebSession, error)
	// CreateWebSession creates a new web session for a user
	CreateWebSession(user string) (types.WebSession, error)

	// AppSession defines application session features.
	services.AppSession
}

// IdentityService manages identities and users
type IdentityService interface {
	// UpsertPassword updates web access password for the user
	UpsertPassword(user string, password []byte) error

	// UpsertOIDCConnector updates or creates OIDC connector
	UpsertOIDCConnector(ctx context.Context, connector services.OIDCConnector) error

	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(ctx context.Context, id string, withSecrets bool) (services.OIDCConnector, error)

	// GetOIDCConnector gets OIDC connectors list
	GetOIDCConnectors(ctx context.Context, withSecrets bool) ([]services.OIDCConnector, error)

	// DeleteOIDCConnector deletes OIDC connector by ID
	DeleteOIDCConnector(ctx context.Context, connectorID string) error

	// CreateOIDCAuthRequest creates OIDCAuthRequest
	CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error)

	// ValidateOIDCAuthCallback validates OIDC auth callback returned from redirect
	ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error)

	// CreateSAMLConnector creates SAML connector
	CreateSAMLConnector(ctx context.Context, connector services.SAMLConnector) error

	// UpsertSAMLConnector updates or creates SAML connector
	UpsertSAMLConnector(ctx context.Context, connector services.SAMLConnector) error

	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(ctx context.Context, id string, withSecrets bool) (services.SAMLConnector, error)

	// GetSAMLConnector gets SAML connectors list
	GetSAMLConnectors(ctx context.Context, withSecrets bool) ([]services.SAMLConnector, error)

	// DeleteSAMLConnector deletes SAML connector by ID
	DeleteSAMLConnector(ctx context.Context, connectorID string) error

	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error)

	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(re string) (*SAMLAuthResponse, error)

	// CreateGithubConnector creates a new Github connector
	CreateGithubConnector(connector services.GithubConnector) error
	// UpsertGithubConnector creates or updates a Github connector
	UpsertGithubConnector(ctx context.Context, connector services.GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(ctx context.Context, withSecrets bool) ([]services.GithubConnector, error)
	// GetGithubConnector returns the specified Github connector
	GetGithubConnector(ctx context.Context, id string, withSecrets bool) (services.GithubConnector, error)
	// DeleteGithubConnector deletes the specified Github connector
	DeleteGithubConnector(ctx context.Context, id string) error
	// CreateGithubAuthRequest creates a new request for Github OAuth2 flow
	CreateGithubAuthRequest(services.GithubAuthRequest) (*services.GithubAuthRequest, error)
	// ValidateGithubAuthCallback validates Github auth callback
	ValidateGithubAuthCallback(q url.Values) (*GithubAuthResponse, error)

	// GetMFAAuthenticateChallenge generates request for user trying to authenticate with U2F token
	GetMFAAuthenticateChallenge(user string, password []byte) (*MFAAuthenticateChallenge, error)

	// GetSignupU2FRegisterRequest generates sign request for user trying to sign up with invite token
	GetSignupU2FRegisterRequest(token string) (*u2f.RegisterChallenge, error)

	// GetUser returns user by name
	GetUser(name string, withSecrets bool) (services.User, error)

	// CreateUser inserts a new entry in a backend.
	CreateUser(ctx context.Context, user services.User) error

	// UpdateUser updates an existing user in a backend.
	UpdateUser(ctx context.Context, user services.User) error

	// UpsertUser user updates or inserts user entry
	UpsertUser(user services.User) error

	// DeleteUser deletes an existng user in a backend by username.
	DeleteUser(ctx context.Context, user string) error

	// GetUsers returns a list of usernames registered in the system
	GetUsers(withSecrets bool) ([]services.User, error)

	// ChangePassword changes user password
	ChangePassword(req services.ChangePasswordReq) error

	// CheckPassword checks if the suplied web access password is valid.
	CheckPassword(user string, password []byte, otpToken string) error

	// GenerateToken creates a special provisioning token for a new SSH server
	// that is valid for ttl period seconds.
	//
	// This token is used by SSH server to authenticate with Auth server
	// and get signed certificate and private key from the auth server.
	//
	// If token is not supplied, it will be auto generated and returned.
	// If TTL is not supplied, token will be valid until removed.
	GenerateToken(ctx context.Context, req GenerateTokenRequest) (string, error)

	// GenerateKeyPair generates SSH private/public key pair optionally protected
	// by password. If the pass parameter is an empty string, the key pair
	// is not password-protected.
	GenerateKeyPair(pass string) ([]byte, []byte, error)

	// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
	// plain text format, signs it using Host Certificate Authority private key and returns the
	// resulting certificate.
	GenerateHostCert(key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error)

	// GenerateUserCerts takes the public key in the OpenSSH `authorized_keys` plain
	// text format, signs it using User Certificate Authority signing key and
	// returns the resulting certificates.
	GenerateUserCerts(ctx context.Context, req proto.UserCertsRequest) (*proto.Certs, error)

	// GenerateUserSingleUseCerts is like GenerateUserCerts but issues a
	// certificate for a single session
	// (https://github.com/gravitational/teleport/blob/3a1cf9111c2698aede2056513337f32bfc16f1f1/rfd/0014-session-2FA.md#sessions).
	GenerateUserSingleUseCerts(ctx context.Context) (proto.AuthService_GenerateUserSingleUseCertsClient, error)

	// IsMFARequiredRequest is a request to check whether MFA is required to
	// access the Target.
	IsMFARequired(ctx context.Context, req *proto.IsMFARequiredRequest) (*proto.IsMFARequiredResponse, error)

	// DeleteAllUsers deletes all users
	DeleteAllUsers() error

	// CreateResetPasswordToken creates a new user reset token
	CreateResetPasswordToken(ctx context.Context, req CreateResetPasswordTokenRequest) (services.ResetPasswordToken, error)

	// ChangePasswordWithToken changes password with token
	ChangePasswordWithToken(ctx context.Context, req ChangePasswordWithTokenRequest) (services.WebSession, error)

	// GetResetPasswordToken returns token
	GetResetPasswordToken(ctx context.Context, username string) (services.ResetPasswordToken, error)

	// RotateResetPasswordTokenSecrets rotates token secrets for a given tokenID
	RotateResetPasswordTokenSecrets(ctx context.Context, tokenID string) (services.ResetPasswordTokenSecrets, error)

	// GetMFADevices fetches all MFA devices registered for the calling user.
	GetMFADevices(ctx context.Context, in *proto.GetMFADevicesRequest) (*proto.GetMFADevicesResponse, error)
	// AddMFADevice adds a new MFA device for the calling user.
	AddMFADevice(ctx context.Context) (proto.AuthService_AddMFADeviceClient, error)
	// DeleteMFADevice deletes a MFA device for the calling user.
	DeleteMFADevice(ctx context.Context) (proto.AuthService_DeleteMFADeviceClient, error)
}

// ProvisioningService is a service in control
// of adding new nodes, auth servers and proxies to the cluster
type ProvisioningService interface {
	// GetTokens returns a list of active invitation tokens for nodes and users
	GetTokens(ctx context.Context, opts ...services.MarshalOption) (tokens []services.ProvisionToken, err error)

	// GetToken returns provisioning token
	GetToken(ctx context.Context, token string) (services.ProvisionToken, error)

	// DeleteToken deletes a given provisioning token on the auth server (CA). It
	// could be a reset password token or a machine token
	DeleteToken(ctx context.Context, token string) error

	// DeleteAllTokens deletes all provisioning tokens
	DeleteAllTokens() error

	// UpsertToken adds provisioning tokens for the auth server
	UpsertToken(ctx context.Context, token services.ProvisionToken) error

	// RegisterUsingToken calls the auth service API to register a new node via registration token
	// which has been previously issued via GenerateToken
	RegisterUsingToken(req RegisterUsingTokenRequest) (*PackedKeys, error)

	// RegisterNewAuthServer is used to register new auth server with token
	RegisterNewAuthServer(ctx context.Context, token string) error
}

// ClientI is a client to Auth service
type ClientI interface {
	IdentityService
	ProvisioningService
	services.Trust
	events.IAuditLog
	events.Streamer
	events.Emitter
	services.Presence
	services.Access
	services.DynamicAccess
	services.DynamicAccessOracle
	WebService
	session.Service
	services.ClusterConfiguration
	services.Events

	types.WebSessionsGetter
	types.WebTokensGetter

	// NewKeepAliver returns a new instance of keep aliver
	NewKeepAliver(ctx context.Context) (services.KeepAliver, error)

	// RotateCertAuthority starts or restarts certificate authority rotation process.
	RotateCertAuthority(req RotateRequest) error

	// RotateExternalCertAuthority rotates external certificate authority,
	// this method is used to update only public keys and certificates of the
	// the certificate authorities of trusted clusters.
	RotateExternalCertAuthority(ca services.CertAuthority) error

	// ValidateTrustedCluster validates trusted cluster token with
	// main cluster, in case if validation is successful, main cluster
	// adds remote cluster
	ValidateTrustedCluster(*ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error)

	// GetDomainName returns auth server cluster name
	GetDomainName() (string, error)

	// GetClusterCACert returns the CAs for the local cluster without signing keys.
	GetClusterCACert() (*LocalCAResponse, error)

	// GenerateServerKeys generates new host private keys and certificates (signed
	// by the host certificate authority) for a node
	GenerateServerKeys(GenerateServerKeysRequest) (*PackedKeys, error)
	// AuthenticateWebUser authenticates web user, creates and  returns web session
	// in case if authentication is successful
	AuthenticateWebUser(req AuthenticateUserRequest) (services.WebSession, error)
	// AuthenticateSSHUser authenticates SSH console user, creates and  returns a pair of signed TLS and SSH
	// short lived certificates as a result
	AuthenticateSSHUser(req AuthenticateSSHRequest) (*SSHLoginResponse, error)

	// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
	// signed certificate if successful.
	ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error)

	// Ping gets basic info about the auth server.
	Ping(ctx context.Context) (proto.PingResponse, error)

	// CreateAppSession creates an application web session. Application web
	// sessions represent a browser session the client holds.
	CreateAppSession(context.Context, services.CreateAppSessionRequest) (services.WebSession, error)

	// GenerateDatabaseCert generates client certificate used by a database
	// service to authenticate with the database instance.
	GenerateDatabaseCert(context.Context, *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)

	// GetWebSession queries the existing web session described with req.
	// Implements ReadAccessPoint.
	GetWebSession(ctx context.Context, req types.GetWebSessionRequest) (types.WebSession, error)

	// GetWebToken queries the existing web token described with req.
	// Implements ReadAccessPoint.
	GetWebToken(ctx context.Context, req types.GetWebTokenRequest) (types.WebToken, error)
}
