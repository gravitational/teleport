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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/roundtrip"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
)

// CurrentVersion is a current API version
const CurrentVersion = "v1"

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

// NewAuthClient returns a new instance of the client which talks to
// an Auth server API (aka "site API") via HTTP-over-SSH
func NewClient(addr string, dialer Dialer) (*Client, error) {
	var params []roundtrip.ClientParam

	if dialer == nil {
		dialer = net.Dial
	}
	transport := &http.Transport{Dial: dialer}
	params = append(params, roundtrip.HTTPClient(&http.Client{
		Transport: transport,
	}))

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
func (c *Client) GetSessions() ([]session.Session, error) {
	out, err := c.Get(c.Endpoint("sessions"), url.Values{})
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
func (c *Client) GetSession(id session.ID) (*session.Session, error) {
	// saving extra round-trip
	if err := id.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("sessions", string(id)), url.Values{})
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
func (c *Client) DeleteSession(id string) error {
	_, err := c.Delete(c.Endpoint("sessions", id))
	return trace.Wrap(err)
}

// CreateSession creates new session
func (c *Client) CreateSession(sess session.Session) error {
	_, err := c.PostJSON(c.Endpoint("sessions"), createSessionReq{Session: sess})
	return trace.Wrap(err)
}

// UpdateSession updates existing session
func (c *Client) UpdateSession(req session.UpdateRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.PutJSON(c.Endpoint("sessions", string(req.ID)), updateSessionReq{Update: req})
	return trace.Wrap(err)
}

// GetLocalDomain returns local auth domain of the current auth server
func (c *Client) GetLocalDomain() (string, error) {
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

// UpsertCertAuthority updates or inserts new cert authority
func (c *Client) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.PostJSON(c.Endpoint("authorities", string(ca.Type)),
		upsertCertAuthorityReq{CA: ca, TTL: ttl})
	return trace.Wrap(err)
}

func (c *Client) GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]*services.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("authorities", string(caType)), url.Values{
		"load_keys": []string{fmt.Sprintf("%t", loadKeys)},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []*services.CertAuthority
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re, nil
}

func (c *Client) DeleteCertAuthority(id services.CertAuthID) error {
	if err := id.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.Delete(c.Endpoint("authorities", string(id.Type), id.DomainName))
	return trace.Wrap(err)
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

// RegisterUserToken calls the auth service API to register a new node via registration token
// which has been previously issued via GenerateToken
func (c *Client) RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error) {
	out, err := c.PostJSON(c.Endpoint("tokens", "register"),
		registerUsingTokenReq{
			HostID: hostID,
			Token:  token,
			Role:   role,
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

// DeleteToken deletes a given provisioning token on the auth server (CA). It
// could be a user token or a machine token
func (c *Client) DeleteToken(token string) error {
	_, err := c.Delete(c.Endpoint("tokens", token))
	return trace.Wrap(err)
}

func (c *Client) RegisterNewAuthServer(token string) error {
	_, err := c.PostJSON(c.Endpoint("tokens", "register", "auth"), registerNewAuthServerReq{
		Token: token,
	})
	return trace.Wrap(err)
}

// UpsertNode is used by SSH servers to reprt their presense
// to the auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertNode(s services.Server, ttl time.Duration) error {
	args := upsertServerReq{
		Server: s,
		TTL:    ttl,
	}
	_, err := c.PostJSON(c.Endpoint("nodes"), args)
	return trace.Wrap(err)
}

// GetNodes returns the list of servers registered in the cluster.
func (c *Client) GetNodes() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("nodes"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []services.Server
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return re, nil
}

// UpsertReverseTunnel is used by admins to create a new reverse tunnel
// to the remote proxy to bypass firewall restrictions
func (c *Client) UpsertReverseTunnel(tunnel services.ReverseTunnel, ttl time.Duration) error {
	args := upsertReverseTunnelReq{
		ReverseTunnel: tunnel,
		TTL:           ttl,
	}
	_, err := c.PostJSON(c.Endpoint("reversetunnels"), args)
	return trace.Wrap(err)
}

// GetReverseTunnels returns the list of created reverse tunnels
func (c *Client) GetReverseTunnels() ([]services.ReverseTunnel, error) {
	out, err := c.Get(c.Endpoint("reversetunnels"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var tunnels []services.ReverseTunnel
	if err := json.Unmarshal(out.Bytes(), &tunnels); err != nil {
		return nil, trace.Wrap(err)
	}
	return tunnels, nil
}

// DeleteReverseTunnel deletes reverse tunnel by domain name
func (c *Client) DeleteReverseTunnel(domainName string) error {
	// this is to avoid confusing error in case if domain emtpy for example
	// HTTP route will fail producing generic not found error
	// instead we catch the error here
	if !cstrings.IsValidDomainName(domainName) {
		return trace.BadParameter("'%v' is not a valid FQDN", domainName)
	}
	_, err := c.Delete(c.Endpoint("reversetunnels", domainName))
	return trace.Wrap(err)
}

// UpsertAuthServer is used by auth servers to report their presense
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertAuthServer(s services.Server, ttl time.Duration) error {
	args := upsertServerReq{
		Server: s,
		TTL:    ttl,
	}
	_, err := c.PostJSON(c.Endpoint("authservers"), args)
	return trace.Wrap(err)
}

// GetAuthServers returns the list of auth servers registered in the cluster.
func (c *Client) GetAuthServers() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("authservers"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []services.Server
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return re, nil
}

// UpsertProxy is used by proxies to report their presense
// to other auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertProxy(s services.Server, ttl time.Duration) error {
	args := upsertServerReq{
		Server: s,
		TTL:    ttl,
	}
	_, err := c.PostJSON(c.Endpoint("proxies"), args)
	return trace.Wrap(err)
}

// GetProxies returns the list of auth servers registered in the cluster.
func (c *Client) GetProxies() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("proxies"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []services.Server
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return re, nil
}

// UpsertPassword updates web access password for the user
func (c *Client) UpsertPassword(user string,
	password []byte) (hotpURL string, hotpQR []byte, err error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "password"),
		upsertPasswordReq{
			Password: string(password),
		})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	var re *upsertPasswordResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", nil, err
	}
	return re.HotpURL, re.HotpQR, err
}

// UpsertUser user updates or inserts user entry
func (c *Client) UpsertUser(user services.User) error {
	_, err := c.PostJSON(c.Endpoint("users"), upsertUserReq{User: user})
	return trace.Wrap(err)
}

// CheckPassword checks if the suplied web access password is valid.
func (c *Client) CheckPassword(user string,
	password []byte, hotpToken string) error {
	_, err := c.PostJSON(
		c.Endpoint("users", user, "web", "password", "check"),
		checkPasswordReq{
			Password:  string(password),
			HOTPToken: hotpToken,
		})
	return trace.Wrap(err)
}

// SignIn checks if the web access password is valid, and if it is valid
// returns a secure web session id.
func (c *Client) SignIn(user string, password []byte) (*Session, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "signin"),
		signInReq{
			Password: string(password),
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// ExtendWebSession creates a new web session for a user based on another
// valid web session
func (c *Client) ExtendWebSession(user string, prevSessionID string) (*Session, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "sessions"),
		createWebSessionReq{
			PrevSessionID: prevSessionID,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// CreateWebSession creates a new web session for a user
func (c *Client) CreateWebSession(user string) (*Session, error) {
	out, err := c.PostJSON(
		c.Endpoint("users", user, "web", "sessions"),
		createWebSessionReq{},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, err
	}
	return sess, nil
}

// GetWebSessionInfo check if a web sesion is valid, returns session id in case if
// it is valid, or error otherwise.
func (c *Client) GetWebSessionInfo(user string, sid string) (*Session, error) {
	out, err := c.Get(
		c.Endpoint("users", user, "web", "sessions", sid), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
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
	user, err := services.GetUserUnmarshaler()(out.Bytes())
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
		user, err := services.GetUserUnmarshaler()(userBytes)
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
	key []byte, hostname, authDomain string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {

	out, err := c.PostJSON(c.Endpoint("ca", "host", "certs"),
		generateHostCertReq{
			Key:        key,
			Hostname:   hostname,
			AuthDomain: authDomain,
			Roles:      roles,
			TTL:        ttl,
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

// GenerateUserCert takes the public key in the Open SSH ``authorized_keys``
// plain text format, signs it using User Certificate Authority signing key and returns the
// resulting certificate.
func (c *Client) GenerateUserCert(
	key []byte, user string, ttl time.Duration) ([]byte, error) {

	out, err := c.PostJSON(c.Endpoint("ca", "user", "certs"),
		generateUserCertReq{
			Key:  key,
			User: user,
			TTL:  ttl,
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

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and hotp generator
func (c *Client) CreateSignupToken(user services.User) (string, error) {
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
func (c *Client) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {

	out, err := c.Get(c.Endpoint("signuptokens", token), url.Values{})
	if err != nil {
		return "", nil, nil, err
	}
	var tokenData getSignupTokenDataResponse
	if err := json.Unmarshal(out.Bytes(), &tokenData); err != nil {
		return "", nil, nil, err
	}
	return tokenData.User, tokenData.QRImg, tokenData.HotpFirstValues, nil
}

// CreateUserWithToken creates account with provided token and password.
// Account username and hotp generator are taken from token data.
// Deletes token after account creation.
func (c *Client) CreateUserWithToken(token, password, hotpToken string) (*Session, error) {
	out, err := c.PostJSON(c.Endpoint("signuptokens", "users"), createUserWithTokenReq{
		Token:     token,
		Password:  password,
		HOTPToken: hotpToken,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var sess *Session
	if err := json.Unmarshal(out.Bytes(), &sess); err != nil {
		return nil, trace.Wrap(err)
	}
	return sess, nil
}

func (c *Client) UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error {
	_, err := c.PostJSON(c.Endpoint("oidc", "connectors"), upsertOIDCConnectorReq{
		Connector: connector,
		TTL:       ttl,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetOIDCConnector(id string, withSecrets bool) (*services.OIDCConnector, error) {
	if id == "" {
		return nil, trace.BadParameter("missing connector id")
	}
	out, err := c.Get(c.Endpoint("oidc", "connectors", id),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var conn *services.OIDCConnector
	if err := json.Unmarshal(out.Bytes(), &conn); err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

func (c *Client) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	out, err := c.Get(c.Endpoint("oidc", "connectors"),
		url.Values{"with_secrets": []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, err
	}
	var connectors []services.OIDCConnector
	if err := json.Unmarshal(out.Bytes(), &connectors); err != nil {
		return nil, trace.Wrap(err)
	}
	return connectors, nil
}

func (c *Client) DeleteOIDCConnector(connectorID string) error {
	if connectorID == "" {
		return trace.BadParameter("missing connector id")
	}
	_, err := c.Delete(c.Endpoint("oidc", "connectors", connectorID))
	return trace.Wrap(err)
}

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

func (c *Client) ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error) {
	out, err := c.PostJSON(c.Endpoint("oidc", "requests", "validate"), validateOIDCAuthCallbackReq{
		Query: q,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response *OIDCAuthResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
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
func (c *Client) PostSessionChunk(sid session.ID, reader io.Reader) error {
	r, err := http.NewRequest("POST", c.Endpoint("sessions", string(sid), "stream"), reader)
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
	var buff [1024]byte
	io.CopyBuffer(ioutil.Discard, re.Body, buff[:])
	return nil
}

// GetSessionChunk allows clients to receive a byte array (chunk) from a recorded
// session stream, starting from 'offset', up to 'max' in length. The upper bound
// of 'max' is set to events.MaxChunkBytes
func (c *Client) GetSessionChunk(sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	response, err := c.Get(c.Endpoint("sessions", string(sid), "stream"), url.Values{
		"offset": []string{strconv.Itoa(offsetBytes)},
		"bytes":  []string{strconv.Itoa(maxBytes)},
	})
	if err != nil {
		logrus.Error(err)
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
func (c *Client) GetSessionEvents(sid session.ID, afterN int) (retval []events.EventFields, err error) {
	query := make(url.Values)
	if afterN > 0 {
		query.Set("after", strconv.Itoa(afterN))
	}
	response, err := c.Get(c.Endpoint("sessions", string(sid), "events"), query)
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

// TOODO(klizhentas) this should be just including appropriate service implementations
type ClientI interface {
	GetUser(name string) (services.User, error)
	UpsertUser(user services.User) error
	GetSessions() ([]session.Session, error)
	GetSession(id session.ID) (*session.Session, error)
	CreateSession(s session.Session) error
	UpdateSession(req session.UpdateRequest) error
	UpsertCertAuthority(cert services.CertAuthority, ttl time.Duration) error
	GetCertAuthorities(caType services.CertAuthType, loadKeys bool) ([]*services.CertAuthority, error)
	DeleteCertAuthority(caType services.CertAuthID) error
	GenerateToken(roles teleport.Roles, ttl time.Duration) (string, error)
	RegisterUsingToken(token, hostID string, role teleport.Role) (*PackedKeys, error)
	RegisterNewAuthServer(token string) error
	UpsertNode(s services.Server, ttl time.Duration) error
	GetNodes() ([]services.Server, error)
	GetAuthServers() ([]services.Server, error)
	UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error)
	CheckPassword(user string, password []byte, hotpToken string) error
	SignIn(user string, password []byte) (*Session, error)
	CreateWebSession(user string) (*Session, error)
	ExtendWebSession(user string, prevSessionID string) (*Session, error)
	GetWebSessionInfo(user string, sid string) (*Session, error)
	DeleteWebSession(user string, sid string) error
	GetUsers() ([]services.User, error)
	DeleteUser(user string) error
	GenerateKeyPair(pass string) ([]byte, []byte, error)
	GenerateHostCert(key []byte, hostname, authServer string, roles teleport.Roles, ttl time.Duration) ([]byte, error)
	GenerateUserCert(key []byte, user string, ttl time.Duration) ([]byte, error)
	GetSignupTokenData(token string) (user string, QRImg []byte, hotpFirstValues []string, e error)
	CreateUserWithToken(token, password, hotpToken string) (*Session, error)
	UpsertOIDCConnector(connector services.OIDCConnector, ttl time.Duration) error
	GetOIDCConnector(id string, withSecrets bool) (*services.OIDCConnector, error)
	GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error)
	DeleteOIDCConnector(connectorID string) error
	CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error)
	ValidateOIDCAuthCallback(q url.Values) (*OIDCAuthResponse, error)

	events.IAuditLog
}
