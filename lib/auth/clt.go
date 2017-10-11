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

package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/roundtrip"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
)

const (
	// CurrentVersion is a current API version
	CurrentVersion = services.V2

	// MissingNamespaceError is a _very_ common error this file generatets
	MissingNamespaceError = "missing required parameter: namespace"
)

type Dialer func(network, addr string) (net.Conn, error)

// Client is HTTP Auth API client. It works by connecting to auth servers
// via HTTP.
//
// When Teleport servers connect to auth API, they usually establish an SSH
// tunnel first, and then do HTTP-over-SSH. This client is wrapped by auth.TunClient
// in lib/auth/tun.go
type Client struct {
	roundtrip.Client
	transport *http.Transport
}

// NewTracer returns request tracer based on the logging level
func NewTracer() roundtrip.RequestTracer {
	if log.GetLevel() >= log.DebugLevel {
		return roundtrip.NewWriterTracer(log.StandardLogger().Writer())
	}
	return roundtrip.NewNopTracer()
}

// NewAuthClient returns a new instance of the client which talks to
// an Auth server API (aka "site API") via HTTP-over-SSH
func NewClient(addr string, dialer Dialer, params ...roundtrip.ClientParam) (*Client, error) {
	if dialer == nil {
		dialer = net.Dial
	}
	transport := &http.Transport{
		Dial: dialer,
		ResponseHeaderTimeout: defaults.DefaultDialTimeout,
	}
	params = append(params,
		roundtrip.HTTPClient(&http.Client{
			Transport: transport,
		}),
		// TODO (ekontsevoy) this tracer pollutes the logs making it harder to work
		// on issues that have nothing to do with the auth API, consider activating it
		// via special environment variable?
		// roundtrip.Tracer(NewTracer),
	)

	c, err := roundtrip.NewClient(addr, CurrentVersion, params...)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:    *c,
		transport: transport,
	}, nil
}

func (c *Client) GetTransport() *http.Transport {
	return c.transport
}

// PostJSON is a generic method that issues http POST request to the server
func (c *Client) PostJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.PostJSON(endpoint, val))
}

// PutJSON is a generic method that issues http PUT request to the server
func (c *Client) PutJSON(
	endpoint string, val interface{}) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.PutJSON(endpoint, val))
}

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(
	endpoint string,
	vals url.Values,
	files ...roundtrip.File) (*roundtrip.Response, error) {

	return httplib.ConvertResponse(c.Client.PostForm(endpoint, vals, files...))
}

// Get issues http GET request to the server
func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.Get(u, params))
}

// Delete issues http Delete Request to the server
func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return httplib.ConvertResponse(c.Client.Delete(u))
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

// DeleteSession deletes a session by ID
func (c *Client) DeleteSession(namespace, id string) error {
	if namespace == "" {
		return trace.BadParameter(MissingNamespaceError)
	}
	_, err := c.Delete(c.Endpoint("namespaces", namespace, "sessions", id))
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

func (c *Client) Close() error {
	return nil
}

// CreateCertAuthority inserts new cert authority
func (c *Client) CreateCertAuthority(ca services.CertAuthority) error {
	return trace.BadParameter("not implemented")
}

// UpsertCertAuthority updates or inserts new cert authority
func (c *Client) UpsertCertAuthority(ca services.CertAuthority) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := services.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("authorities", string(ca.GetType())),
		&upsertCertAuthorityRawReq{CA: data})
	return trace.Wrap(err)
}

// GetCertAuthorities returns a list of certificate authorities
func (c *Client) GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]services.CertAuthority, error) {
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
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = ca
	}
	return re, nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *Client) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool) (services.CertAuthority, error) {
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("authorities", string(id.Type), id.DomainName), url.Values{
		"load_keys": []string{fmt.Sprintf("%t", loadSigningKeys)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(out.Bytes())
}

// DeleteCertAuthority deletes cert authority by ID
func (c *Client) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.Delete(c.Endpoint("authorities", string(id.Type), id.DomainName))
	return trace.Wrap(err)
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (c *Client) ActivateCertAuthority(id services.CertAuthID) error {
	return trace.BadParameter("not implemented")
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (c *Client) DeactivateCertAuthority(id services.CertAuthID) error {
	return trace.BadParameter("not implemented")
}

// GenerateToken creates a special provisioning token for a new SSH server
// that is valid for ttl period seconds.
//
// This token is used by SSH server to authenticate with Auth server
// and get signed certificate and private key from the auth server.
//
// The token can be used only once.
func (c *Client) GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error) {
	out, err := c.PostJSON(c.Endpoint("tokens"), generateTokenReq{
		Roles: roles,
		TTL:   ttl,
	})
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
func (c *Client) RegisterUsingToken(token, hostID string, nodeName string, role teleport.Role) (*PackedKeys, error) {
	out, err := c.PostJSON(c.Endpoint("tokens", "register"),
		registerUsingTokenReq{
			HostID:   hostID,
			NodeName: nodeName,
			Token:    token,
			Role:     role,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var keys PackedKeys
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, trace.Wrap(err)
	}

	return &keys, nil
}

// GetTokens returns a list of active invitation tokens for nodes and users
func (c *Client) GetTokens() (tokens []services.ProvisionToken, err error) {
	out, err := c.Get(c.Endpoint("tokens"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := json.Unmarshal(out.Bytes(), &tokens); err != nil {
		return nil, trace.Wrap(err)
	}
	return tokens, nil
}

// GetToken returns provisioning token
func (c *Client) GetToken(token string) (*services.ProvisionToken, error) {
	out, err := c.Get(c.Endpoint("tokens", token), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tok services.ProvisionToken
	if err := json.Unmarshal(out.Bytes(), &tok); err != nil {
		return nil, trace.Wrap(err)
	}
	return &tok, nil
}

// DeleteToken deletes a given provisioning token on the auth server (CA). It
// could be a user token or a machine token
func (c *Client) DeleteToken(token string) error {
	_, err := c.Delete(c.Endpoint("tokens", token))
	return trace.Wrap(err)
}

// RegisterNewAuthServer is used to register new auth server with token
func (c *Client) RegisterNewAuthServer(token string) error {
	_, err := c.PostJSON(c.Endpoint("tokens", "register", "auth"), registerNewAuthServerReq{
		Token: token,
	})
	return trace.Wrap(err)
}

// UpsertNode is used by SSH servers to reprt their presense
// to the auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertNode(s services.Server) error {
	if s.GetNamespace() == "" {
		return trace.BadParameter("missing node namespace")
	}
	data, err := services.GetServerMarshaler().MarshalServer(s)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertServerRawReq{
		Server: data,
	}
	_, err = c.PostJSON(c.Endpoint("namespaces", s.GetNamespace(), "nodes"), args)
	return trace.Wrap(err)
}

// GetNodes returns the list of servers registered in the cluster.
func (c *Client) GetNodes(namespace string) ([]services.Server, error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	out, err := c.Get(c.Endpoint("namespaces", namespace, "nodes"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	re := make([]services.Server, len(items))
	for i, raw := range items {
		s, err := services.GetServerMarshaler().UnmarshalServer(raw, services.KindNode)
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
	data, err := services.GetReverseTunnelMarshaler().MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	args := &upsertReverseTunnelRawReq{
		ReverseTunnel: data,
	}
	_, err = c.PostJSON(c.Endpoint("reversetunnels"), args)
	return trace.Wrap(err)
}

// GetReverseTunnels returns the list of created reverse tunnels
func (c *Client) GetReverseTunnels() ([]services.ReverseTunnel, error) {
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
		tunnel, err := services.GetReverseTunnelMarshaler().UnmarshalReverseTunnel(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tunnels[i] = tunnel
	}
	return tunnels, nil
}

// DeleteReverseTunnel deletes reverse tunnel by domain name
func (c *Client) DeleteReverseTunnel(domainName string) error {
	// this is to avoid confusing error in case if domain emtpy for example
	// HTTP route will fail producing generic not found error
	// instead we catch the error here
	if strings.TrimSpace(domainName) == "" {
		return trace.BadParameter("empty domain name")
	}
	_, err := c.Delete(c.Endpoint("reversetunnels", domainName))
	return trace.Wrap(err)
}

// UpsertAuthServer is used by auth servers to report their presense
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertAuthServer(s services.Server) error {
	data, err := services.GetServerMarshaler().MarshalServer(s)
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
		server, err := services.GetServerMarshaler().UnmarshalServer(raw, services.KindAuthServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
}

// UpsertProxy is used by proxies to report their presense
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertProxy(s services.Server) error {
	data, err := services.GetServerMarshaler().MarshalServer(s)
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
		server, err := services.GetServerMarshaler().UnmarshalServer(raw, services.KindProxy)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		re[i] = server
	}
	return re, nil
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

// UpsertUser user updates or inserts user entry
func (c *Client) UpsertUser(user services.User) error {
	data, err := services.GetUserMarshaler().MarshalUser(user)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("users"), &upsertUserRawReq{User: data})
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

// SignIn checks if the web access password is valid, and if it is valid
// returns a secure web session id.
func (c *Client) SignIn(user string, password []byte) (services.WebSession, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "signin"),
		signInReq{
			Password: string(password),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// PreAuthenticatedSignIn is for 2-way authentication methods like U2F where the password is
// already checked before issueing the second factor challenge
func (c *Client) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	out, err := c.Get(
		c.Endpoint("users", user, "web", "signin", "preauth"),
		url.Values{},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// GetU2FSignRequest generates request for user trying to authenticate with U2F token
func (c *Client) GetU2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	out, err := c.PostJSON(
		c.Endpoint("u2f", "users", user, "sign"),
		signInReq{
			Password: string(password),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var signRequest *u2f.SignRequest
	if err := json.Unmarshal(out.Bytes(), &signRequest); err != nil {
		return nil, err
	}
	return signRequest, nil
}

// ExtendWebSession creates a new web session for a user based on another
// valid web session
func (c *Client) ExtendWebSession(user string, prevSessionID string) (services.WebSession, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "sessions"),
		createWebSessionReq{
			PrevSessionID: prevSessionID,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
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
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// GetWebSessionInfo checks if a web sesion is valid, returns session id in case if
// it is valid, or error otherwise.
func (c *Client) GetWebSessionInfo(user string, sid string) (services.WebSession, error) {
	out, err := c.Get(
		c.Endpoint("users", user, "web", "sessions", sid), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// DeleteWebSession deletes a web session for this user by id
func (c *Client) DeleteWebSession(user string, sid string) error {
	_, err := c.Delete(c.Endpoint("users", user, "web", "sessions", sid))
	return trace.Wrap(err)
}

// GetUser returns a list of usernames registered in the system
func (c *Client) GetUser(name string) (services.User, error) {
	if name == "" {
		return nil, trace.BadParameter("missing username")
	}
	out, err := c.Get(c.Endpoint("users", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.GetUserMarshaler().UnmarshalUser(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// GetUsers returns a list of usernames registered in the system
func (c *Client) GetUsers() ([]services.User, error) {
	out, err := c.Get(c.Endpoint("users"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	users := make([]services.User, len(items))
	for i, userBytes := range items {
		user, err := services.GetUserMarshaler().UnmarshalUser(userBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		users[i] = user
	}
	return users, nil
}

// DeleteUser deletes a user by username
func (c *Client) DeleteUser(user string) error {
	_, err := c.Delete(c.Endpoint("users", user))
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
	key []byte, hostID, nodeName, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {

	out, err := c.PostJSON(c.Endpoint("ca", "host", "certs"),
		generateHostCertReq{
			Key:         key,
			HostID:      hostID,
			NodeName:    nodeName,
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

// GenerateUserCert takes the public key in the OpenSSH `authorized_keys` plain
// text format, signs it using User Certificate Authority signing key and
// returns the resulting certificate.
func (c *Client) GenerateUserCert(key []byte, user string, ttl time.Duration, compatibility string) ([]byte, error) {
	out, err := c.PostJSON(c.Endpoint("ca", "user", "certs"),
		generateUserCertReq{
			Key:           key,
			User:          user,
			TTL:           ttl,
			Compatibility: compatibility,
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cert string
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, trace.Wrap(err)
	}
	return []byte(cert), nil
}

// GenerateUserCertBundle takes the public key in the OpenSSH `authorized_keys`
// plain text format, signs it using User Certificate Authority signing key and
// returns the resulting certificate. It also includes the host certificate that
// can be added to the known_hosts file.
func (c *Client) GenerateUserCertBundle(key []byte, user string, ttl time.Duration, compatibility string) ([]byte, []services.CertAuthorityV1, error) {
	out, err := c.PostJSON(c.Endpoint("ca", "user", "certs", "bundle"),
		generateUserCertReq{
			Key:           key,
			User:          user,
			TTL:           ttl,
			Compatibility: compatibility,
		})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var br sshUserCertBundleResponse
	if err := json.Unmarshal(out.Bytes(), &br); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return br.Cert, br.HostSigners, nil
}

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and otp generator
func (c *Client) CreateSignupToken(user services.UserV1) (string, error) {
	if err := user.Check(); err != nil {
		return "", trace.Wrap(err)
	}
	out, err := c.PostJSON(c.Endpoint("signuptokens"), createSignupTokenReq{
		User: user,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	var token string
	if err := json.Unmarshal(out.Bytes(), &token); err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

// GetSignupTokenData returns token data for a valid token
func (c *Client) GetSignupTokenData(token string) (user string, otpQRCode []byte, e error) {
	out, err := c.Get(c.Endpoint("signuptokens", token), url.Values{})
	if err != nil {
		return "", nil, err
	}

	var tokenData getSignupTokenDataResponse
	if err := json.Unmarshal(out.Bytes(), &tokenData); err != nil {
		return "", nil, err
	}

	return tokenData.User, tokenData.QRImg, nil
}

// GetSignupU2FRegisterRequest generates sign request for user trying to sign up with invite tokenx
func (c *Client) GetSignupU2FRegisterRequest(token string) (u2fRegisterRequest *u2f.RegisterRequest, e error) {
	out, err := c.Get(c.Endpoint("u2f", "signuptokens", token), url.Values{})
	if err != nil {
		return nil, err
	}
	var u2fRegReq u2f.RegisterRequest
	if err := json.Unmarshal(out.Bytes(), &u2fRegReq); err != nil {
		return nil, err
	}
	return &u2fRegReq, nil
}

// CreateUserWithOTP creates account with provided token and password.
// Account username and OTP key are taken from token data.
// Deletes token after account creation.
func (c *Client) CreateUserWithOTP(token, password, otpToken string) (services.WebSession, error) {
	out, err := c.PostJSON(c.Endpoint("signuptokens", "users"), createUserWithTokenReq{
		Token:    token,
		Password: password,
		OTPToken: otpToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// CreateUserWithoutOTP validates a given token creates a user
// with the given password and deletes the token afterwards.
func (c *Client) CreateUserWithoutOTP(token string, password string) (services.WebSession, error) {
	out, err := c.PostJSON(c.Endpoint("signuptokens", "users"), createUserWithTokenReq{
		Token:    token,
		Password: password,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// CreateUserWithU2FToken creates user account with provided token and U2F sign response
func (c *Client) CreateUserWithU2FToken(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (services.WebSession, error) {
	out, err := c.PostJSON(c.Endpoint("u2f", "users"), createUserWithU2FTokenReq{
		Token:               token,
		Password:            password,
		U2FRegisterResponse: u2fRegisterResponse,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetWebSessionMarshaler().UnmarshalWebSession(out.Bytes())
}

// UpsertOIDCConnector updates or creates OIDC connector
func (c *Client) UpsertOIDCConnector(connector services.OIDCConnector) error {
	data, err := services.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
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
func (c *Client) GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error) {
	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("oidc", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	return services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(out.Bytes())
}

// GetOIDCConnector gets OIDC connectors list
func (c *Client) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
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
		connector, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteOIDCConnector deletes OIDC connector by ID
func (c *Client) DeleteOIDCConnector(connectorID string) error {
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
	}
	if len(rawResponse.Session) != 0 {
		session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(rawResponse.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	response.HostSigners = make([]services.CertAuthority, len(rawResponse.HostSigners))
	for i, raw := range rawResponse.HostSigners {
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners[i] = ca
	}
	return &response, nil
}

// CreateOIDCConnector creates SAML connector
func (c *Client) CreateSAMLConnector(connector services.SAMLConnector) error {
	data, err := services.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
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

// UpsertSAMLConnector updates or creates OIDC connector
func (c *Client) UpsertSAMLConnector(connector services.SAMLConnector) error {
	data, err := services.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("saml", "connectors"), &upsertSAMLConnectorRawReq{
		Connector: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCConnector returns SAML connector information by id
func (c *Client) GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error) {
	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("saml", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(out.Bytes())
}

// GetSAMLConnectors gets SAML connectors list
func (c *Client) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
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
		connector, err := services.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteSAMLConnector deletes SAML connector by ID
func (c *Client) DeleteSAMLConnector(connectorID string) error {
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
	}
	if len(rawResponse.Session) != 0 {
		session, err := services.GetWebSessionMarshaler().UnmarshalWebSession(rawResponse.Session)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.Session = session
	}
	response.HostSigners = make([]services.CertAuthority, len(rawResponse.HostSigners))
	for i, raw := range rawResponse.HostSigners {
		ca, err := services.GetCertAuthorityMarshaler().UnmarshalCertAuthority(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		response.HostSigners[i] = ca
	}
	return &response, nil
}

// EmitAuditEvent sends an auditable event to the auth server (part of evets.IAuditLog interface)
func (c *Client) EmitAuditEvent(eventType string, fields events.EventFields) error {
	_, err := c.PostJSON(c.Endpoint("events"), &auditEventReq{
		Type:   eventType,
		Fields: fields,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostSessionChunk allows clients to submit session stream chunks to the audit log
// (part of evets.IAuditLog interface)
//
// The data is POSTed to HTTP server as a simple binary body (no encodings of any
// kind are needed)
func (c *Client) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	if namespace == "" {
		return trace.BadParameter(MissingNamespaceError)
	}
	r, err := http.NewRequest("POST", c.Endpoint("namespaces", namespace, "sessions", string(sid), "stream"), reader)
	if err != nil {
		return trace.Wrap(err)
	}
	r.Header.Set("Content-Type", "application/octet-stream")
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
	r, err := http.NewRequest("POST", c.Endpoint("namespaces", slice.Namespace, "sessions", string(slice.SessionID), "slice"), bytes.NewReader(data))
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

// Returns events that happen during a session sorted by time
// (oldest first).
//
// afterN allows to filter by "newer than N" value where N is the cursor ID
// of previously returned bunch (good for polling for latest)
//
// This function is usually used in conjunction with GetSessionReader to
// replay recorded session streams.
func (c *Client) GetSessionEvents(namespace string, sid session.ID, afterN int) (retval []events.EventFields, err error) {
	if namespace == "" {
		return nil, trace.BadParameter(MissingNamespaceError)
	}
	query := make(url.Values)
	if afterN > 0 {
		query.Set("after", strconv.Itoa(afterN))
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
func (c *Client) SearchEvents(from, to time.Time, query string) ([]events.EventFields, error) {
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, trace.BadParameter("query")
	}
	q.Set("from", from.Format(time.RFC3339))
	q.Set("to", to.Format(time.RFC3339))
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
func (c *Client) SearchSessionEvents(from, to time.Time) ([]events.EventFields, error) {
	query := url.Values{
		"to":   []string{to.Format(time.RFC3339)},
		"from": []string{from.Format(time.RFC3339)},
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
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
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
	var ns services.Namespace
	if err := json.Unmarshal(out.Bytes(), &ns); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ns, nil
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
func (c *Client) GetRoles() ([]services.Role, error) {
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
		role, err := services.GetRoleMarshaler().UnmarshalRole(roleBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles[i] = role
	}
	return roles, nil
}

// CreateRole creates a role.
func (c *Client) CreateRole(role services.Role, ttl time.Duration) error {
	return trace.BadParameter("not implemented")
}

// UpsertRole creates or updates role
func (c *Client) UpsertRole(role services.Role, ttl time.Duration) error {
	data, err := services.GetRoleMarshaler().MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("roles"), &upsertRoleRawReq{Role: data})
	return trace.Wrap(err)
}

// GetRole returns role by name
func (c *Client) GetRole(name string) (services.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing name")
	}
	out, err := c.Get(c.Endpoint("roles", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := services.GetRoleMarshaler().UnmarshalRole(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return role, nil
}

// DeleteRole deletes role by name
func (c *Client) DeleteRole(name string) error {
	_, err := c.Delete(c.Endpoint("roles", name))
	return trace.Wrap(err)
}

func (c *Client) GetClusterName() (services.ClusterName, error) {
	out, err := c.Get(c.Endpoint("configuration", "name"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cn, err := services.GetClusterNameMarshaler().Unmarshal(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cn, err
}

func (c *Client) SetClusterName(cn services.ClusterName) error {
	data, err := services.GetClusterNameMarshaler().Marshal(cn)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("configuration", "name"), &setClusterNameReq{ClusterName: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *Client) GetStaticTokens() (services.StaticTokens, error) {
	out, err := c.Get(c.Endpoint("configuration", "static_tokens"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	st, err := services.GetStaticTokensMarshaler().Unmarshal(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return st, err
}

func (c *Client) SetStaticTokens(st services.StaticTokens) error {
	data, err := services.GetStaticTokensMarshaler().Marshal(st)
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

	cap, err := services.GetAuthPreferenceMarshaler().Unmarshal(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cap, nil
}

func (c *Client) SetAuthPreference(cap services.AuthPreference) error {
	data, err := services.GetAuthPreferenceMarshaler().Marshal(cap)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("authentication", "preference"), &setClusterAuthPreferenceReq{ClusterAuthPreference: data})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetLocalClusterName returns local cluster name
func (c *Client) GetLocalClusterName() (string, error) {
	return c.GetDomainName()
}

// UpsertLocalClusterName upserts local cluster name
func (c *Client) UpsertLocalClusterName(string) error {
	return trace.BadParameter("not implemented")
}

// DeleteAllCertAuthorities deletes all certificate authorities of a certain type
func (c *Client) DeleteAllCertAuthorities(caType services.CertAuthType) error {
	return trace.BadParameter("not implemented")
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (c *Client) DeleteAllReverseTunnels() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllCertNamespaces deletes all namespaces
func (c *Client) DeleteAllNamespaces() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllProxies deletes all proxies
func (c *Client) DeleteAllProxies() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllNodes deletes all nodes in a given namespace
func (c *Client) DeleteAllNodes(namespace string) error {
	return trace.BadParameter("not implemented")
}

// DeleteAllRoles deletes all roles
func (c *Client) DeleteAllRoles() error {
	return trace.BadParameter("not implemented")
}

// DeleteAllUsers deletes all users
func (c *Client) DeleteAllUsers() error {
	return trace.BadParameter("not implemented")
}

func (c *Client) GetTrustedCluster(name string) (services.TrustedCluster, error) {
	out, err := c.Get(c.Endpoint("trustedclusters", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedCluster, err := services.GetTrustedClusterMarshaler().Unmarshal(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return trustedCluster, nil
}

func (c *Client) GetTrustedClusters() ([]services.TrustedCluster, error) {
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
		trustedCluster, err := services.GetTrustedClusterMarshaler().Unmarshal(bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		trustedClusters[i] = trustedCluster
	}

	return trustedClusters, nil
}

func (c *Client) UpsertTrustedCluster(trustedCluster services.TrustedCluster) error {
	trustedClusterBytes, err := services.GetTrustedClusterMarshaler().Marshal(trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = c.PostJSON(c.Endpoint("trustedclusters"), &upsertTrustedClusterReq{
		TrustedCluster: trustedClusterBytes,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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

func (c *Client) DeleteTrustedCluster(name string) error {
	_, err := c.Delete(c.Endpoint("trustedclusters", name))
	return trace.Wrap(err)
}

// WebService implements features used by Web UI clients
type WebService interface {
	// GetWebSessionInfo checks if a web sesion is valid, returns session id in case if
	// it is valid, or error otherwise.
	GetWebSessionInfo(user string, sid string) (services.WebSession, error)
	// ExtendWebSession creates a new web session for a user based on another
	// valid web session
	ExtendWebSession(user string, prevSessionID string) (services.WebSession, error)
	// CreateWebSession creates a new web session for a user
	CreateWebSession(user string) (services.WebSession, error)
	// DeleteWebSession deletes a web session for this user by id
	DeleteWebSession(user string, sid string) error
}

// IdentityService manages identities and users
type IdentityService interface {
	// UpsertPassword updates web access password for the user
	UpsertPassword(user string, password []byte) error

	// UpsertOIDCConnector updates or creates OIDC connector
	UpsertOIDCConnector(connector services.OIDCConnector) error

	// GetOIDCConnector returns OIDC connector information by id
	GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error)

	// GetOIDCConnector gets OIDC connectors list
	GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error)

	// DeleteOIDCConnector deletes OIDC connector by ID
	DeleteOIDCConnector(connectorID string) error

	// CreateOIDCAuthRequest creates OIDCAuthRequest
	CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error)

	// ValidateOIDCAuthCallback validates OIDC auth callback returned from redirect
	ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error)

	// CreateSAMLConnector creates SAML connector
	CreateSAMLConnector(connector services.SAMLConnector) error

	// UpsertSAMLConnector updates or creates SAML connector
	UpsertSAMLConnector(connector services.SAMLConnector) error

	// GetSAMLConnector returns SAML connector information by id
	GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error)

	// GetSAMLConnector gets SAML connectors list
	GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error)

	// DeleteSAMLConnector deletes SAML connector by ID
	DeleteSAMLConnector(connectorID string) error

	// CreateSAMLAuthRequest creates SAML AuthnRequest
	CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error)

	// ValidateSAMLResponse validates SAML auth response
	ValidateSAMLResponse(re string) (*SAMLAuthResponse, error)

	// GetU2FSignRequest generates request for user trying to authenticate with U2F token
	GetU2FSignRequest(user string, password []byte) (*u2f.SignRequest, error)

	// GetSignupU2FRegisterRequest generates sign request for user trying to sign up with invite token
	GetSignupU2FRegisterRequest(token string) (*u2f.RegisterRequest, error)

	// CreateUserWithU2FToken creates user account with provided token and U2F sign response
	CreateUserWithU2FToken(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (services.WebSession, error)

	// PreAuthenticatedSignIn is used get web session for a user that is already authenticated
	PreAuthenticatedSignIn(user string) (services.WebSession, error)

	// GetUser returns user by name
	GetUser(name string) (services.User, error)

	// UpsertUser user updates or inserts user entry
	UpsertUser(user services.User) error

	// DeleteUser deletes a user by username
	DeleteUser(user string) error

	// GetUsers returns a list of usernames registered in the system
	GetUsers() ([]services.User, error)

	// CheckPassword checks if the suplied web access password is valid.
	CheckPassword(user string, password []byte, otpToken string) error

	// SignIn checks if the web access password is valid, and if it is valid
	// returns a secure web session id.
	SignIn(user string, password []byte) (services.WebSession, error)

	// CreateUserWithOTP creates account with provided token and password.
	// Account username and OTP key are taken from token data.
	// Deletes token after account creation.
	CreateUserWithOTP(token, password, otpToken string) (services.WebSession, error)

	// CreateUserWithoutOTP validates a given token creates a user
	// with the given password and deletes the token afterwards.
	CreateUserWithoutOTP(token string, password string) (services.WebSession, error)

	// GenerateToken creates a special provisioning token for a new SSH server
	// that is valid for ttl period seconds.
	//
	// This token is used by SSH server to authenticate with Auth server
	// and get signed certificate and private key from the auth server.
	//
	// The token can be used only once.
	GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error)

	// GenerateKeyPair generates SSH private/public key pair optionally protected
	// by password. If the pass parameter is an empty string, the key pair
	// is not password-protected.
	GenerateKeyPair(pass string) ([]byte, []byte, error)

	// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
	// plain text format, signs it using Host Certificate Authority private key and returns the
	// resulting certificate.
	GenerateHostCert(key []byte, hostID, nodeName, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error)

	// GenerateUserCert takes the public key in the OpenSSH `authorized_keys`
	// plain text format, signs it using User Certificate Authority signing key and returns the
	// resulting certificate.
	GenerateUserCert(key []byte, user string, ttl time.Duration, compatibility string) ([]byte, error)

	// GetSignupTokenData returns token data for a valid token
	GetSignupTokenData(token string) (user string, otpQRCode []byte, e error)

	// CreateSignupToken creates one time token for creating account for the user
	// For each token it creates username and OTP key
	CreateSignupToken(user services.UserV1) (string, error)
}

// ProvisioningService is a service in control
// of adding new nodes, auth servers and proxies to the cluster
type ProvisioningService interface {
	// GetTokens returns a list of active invitation tokens for nodes and users
	GetTokens() (tokens []services.ProvisionToken, err error)

	// GetToken returns provisioning token
	GetToken(token string) (*services.ProvisionToken, error)

	// DeleteToken deletes a given provisioning token on the auth server (CA). It
	// could be a user token or a machine token
	DeleteToken(token string) error

	// RegisterUsingToken calls the auth service API to register a new node via registration token
	// which has been previously issued via GenerateToken
	RegisterUsingToken(token, hostID string, nodeName string, role teleport.Role) (*PackedKeys, error)

	// RegisterNewAuthServer is used to register new auth server with token
	RegisterNewAuthServer(token string) error
}

// ClientI is a client to Auth service
type ClientI interface {
	IdentityService
	ProvisioningService
	services.Trust
	events.IAuditLog
	services.Presence
	services.Access
	WebService
	session.Service
	services.ClusterConfiguration

	ValidateTrustedCluster(*ValidateTrustedClusterRequest) (*ValidateTrustedClusterResponse, error)
	GetDomainName() (string, error)
}
