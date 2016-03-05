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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/codahale/lunk"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
)

// CurrentVersion is a current API version
const CurrentVersion = "v1"

// Client is HTTP API client that connects to the remote server
type Client struct {
	roundtrip.Client
}

// NewClientFromNetAddr returns a new instance of the client
func NewClientFromNetAddr(
	a utils.NetAddr, params ...roundtrip.ClientParam) (*Client, error) {

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, address string) (net.Conn, error) {
				return net.Dial(a.AddrNetwork, a.Addr)
			}}}
	params = append(params, roundtrip.HTTPClient(client))
	u := url.URL{
		Scheme: "http",
		Host:   "placeholder",
		Path:   a.Path,
	}
	return NewClient(u.String(), params...)
}

// NewClient returns a new instance of the client
func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion, params...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
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
func (c *Client) GetSession(id string) (*session.Session, error) {
	out, err := c.Get(c.Endpoint("sessions", id), url.Values{})
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
	_, err := c.PutJSON(c.Endpoint("sessions", req.ID), updateSessionReq{Update: req})
	return trace.Wrap(err)
}

// UpsertParty updates existing session party or inserts new party
func (c *Client) UpsertParty(id string, p session.Party, ttl time.Duration) error {
	_, err := c.PostJSON(c.Endpoint("sessions", id, "parties"), upsertPartyReq{Party: p, TTL: ttl})
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

// UpsertCertAuthority updates or inserts new cert authority
func (c *Client) UpsertCertAuthority(ca services.CertAuthority, ttl time.Duration) error {
	if err := ca.Check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := c.PostJSON(c.Endpoint("authorities", string(ca.Type)),
		upsertCertAuthorityReq{CA: ca, TTL: ttl})
	return trace.Wrap(err)
}

func (c *Client) GetCertAuthorities(caType services.CertAuthType) ([]*services.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("authorities", string(caType)), url.Values{})
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
func (c *Client) GenerateToken(role teleport.Role, ttl time.Duration) (string, error) {
	out, err := c.PostJSON(c.Endpoint("tokens"), generateTokenReq{
		Role: role,
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
func (c *Client) RegisterUsingToken(token, hostID string, role teleport.Role) (PackedKeys, error) {
	out, err := c.PostJSON(c.Endpoint("tokens", "register"),
		registerUsingTokenReq{
			HostID: hostID,
			Token:  token,
			Role:   role,
		})
	if err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	var keys PackedKeys
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return PackedKeys{}, trace.Wrap(err)
	}
	return keys, nil
}

func (c *Client) RegisterNewAuthServer(nodename, token string,
	publicSealKey encryptor.Key) (masterKey encryptor.Key, e error) {

	out, err := c.PostJSON(c.Endpoint("tokens", "register", "auth"), registerNewAuthServerReq{
		Token: token,
		Key:   publicSealKey,
	})
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	if err := json.Unmarshal(out.Bytes(), &masterKey); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	return masterKey, nil
}

func (c *Client) Log(id lunk.EventID, e lunk.Event) {
	en := lunk.NewEntry(id, e)
	en.Time = time.Now()
	c.LogEntry(en)
}

func (c *Client) LogEntry(en lunk.Entry) error {
	_, err := c.PostJSON(c.Endpoint("events"), submitEventsReq{Events: []lunk.Entry{en}})
	return trace.Wrap(err)
}

func (c *Client) LogSession(sess session.Session) error {
	_, err := c.PostJSON(c.Endpoint("events", "sessions"), logSessionsReq{Sessions: []session.Session{sess}})
	return trace.Wrap(err)
}

func (c *Client) GetEvents(filter events.Filter) ([]lunk.Entry, error) {
	vals, err := events.FilterToURL(filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("events"), vals)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var events []lunk.Entry
	if err := json.Unmarshal(out.Bytes(), &events); err != nil {
		return nil, trace.Wrap(err)
	}
	return events, nil
}

func (c *Client) GetSessionEvents(filter events.Filter) ([]session.Session, error) {
	vals, err := events.FilterToURL(filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.Get(c.Endpoint("events", "sessions"), vals)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var events []session.Session
	if err := json.Unmarshal(out.Bytes(), &events); err != nil {
		return nil, trace.Wrap(err)
	}
	return events, nil
}

func (c *Client) GetChunkWriter(id string) (recorder.ChunkWriteCloser, error) {
	return &chunkRW{c: c, id: id}, nil
}

func (c *Client) GetChunkReader(id string) (recorder.ChunkReadCloser, error) {
	return &chunkRW{c: c, id: id}, nil
}

// UpsertServer is used by SSH servers to reprt their presense
// to the auth servers in form of hearbeat expiring after ttl period.
func (c *Client) UpsertServer(s services.Server, ttl time.Duration) error {
	args := upsertServerReq{
		Server: s,
		TTL:    ttl,
	}
	_, err := c.PostJSON(c.Endpoint("servers"), args)
	return trace.Wrap(err)
}

// GetServers returns the list of servers registered in the cluster.
func (c *Client) GetServers() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("servers"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var re []services.Server
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, trace.Wrap(err)
	}
	return re, nil
}

// GetAuthServers returns the list of auth servers registered in the cluster.
func (c *Client) GetAuthServers() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("auth", "servers"), url.Values{})
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

// CreateWebSession creates a new web session for a user based on another
// valid web session
func (c *Client) CreateWebSession(user string, prevSessionID string) (*Session, error) {
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

// GetWebSessionKeys returns the list of temporary keys generated for this
// user web session. Each web session has a temporary user ssh key and
// certificate generated, that is stored for the duration of this web
// session. These keys are used to access SSH servers via web portal.
func (c *Client) GetWebSessionsKeys(
	user string) ([]services.AuthorizedKey, error) {

	out, err := c.Get(c.Endpoint("users", user, "web", "sessions"), url.Values{})
	if err != nil {
		return nil, err
	}
	var keys []services.AuthorizedKey
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// DeleteWebSession deletes a web session for this user by id
func (c *Client) DeleteWebSession(user string, sid string) error {
	_, err := c.Delete(c.Endpoint("users", user, "web", "sessions", sid))
	return trace.Wrap(err)
}

// GetUsers returns a list of usernames registered in the system
func (c *Client) GetUsers() ([]services.User, error) {
	out, err := c.Get(c.Endpoint("users"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var users []services.User
	if err := json.Unmarshal(out.Bytes(), &users); err != nil {
		return nil, trace.Wrap(err)
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
	key []byte, hostname, authDomain string, role teleport.Role, ttl time.Duration) ([]byte, error) {

	out, err := c.PostJSON(c.Endpoint("ca", "host", "certs"),
		generateHostCertReq{
			Key:        key,
			Hostname:   hostname,
			AuthDomain: authDomain,
			Role:       role,
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

// GetSealKeys returns IDs of all the backend encrypting keys that
// this server has
func (c *Client) GetSealKeys() ([]encryptor.Key, error) {
	out, err := c.Get(c.Endpoint("backend", "keys"), url.Values{})
	if err != nil {
		return nil, err
	}
	var keys []encryptor.Key
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, err
	}
	return keys, err
}

// GenerateSealKey generates a new backend encrypting key with the
// given id and then backend makes a copy of all the data using the
// generated key for encryption
func (c *Client) GenerateSealKey(keyName string) (encryptor.Key, error) {
	out, err := c.PostJSON(c.Endpoint("backend", "generatekey"), generateSealKeyReq{
		Name: keyName})
	if err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}

	var key encryptor.Key
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	return key, nil
}

// DeleteSealKey deletes the backend encrypting key and all the data
// encrypted with the key
func (c *Client) DeleteSealKey(keyID string) error {
	_, err := c.Delete(c.Endpoint("backend", "keys", keyID))
	return trace.Wrap(err)
}

// AddSealKey adds the given encrypting key. If backend works not in
// readonly mode, backend makes a copy of the data using the key for
// encryption
func (c *Client) AddSealKey(key encryptor.Key) error {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostForm(c.Endpoint("backend", "keys"), url.Values{
		"key": []string{string(keyJSON)}})
	return trace.Wrap(err)
}

// GetSealKey returns the backend encrypting key.
func (c *Client) GetSealKey(keyID string) (encryptor.Key, error) {
	out, err := c.Get(c.Endpoint("backend", "keys", keyID), url.Values{})
	if err != nil {
		return encryptor.Key{}, err
	}
	var key encryptor.Key
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return encryptor.Key{}, trace.Wrap(err)
	}
	return key, nil
}

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and hotp generator
func (c *Client) CreateSignupToken(user string, allowedLogins []string) (string, error) {
	if len(allowedLogins) == 0 {
		// TODO(klizhentas) do validation on the serverside
		return "", trace.Wrap(
			teleport.BadParameter("allowedUsers",
				"cannot create a new account without any allowed logins"))
	}
	out, err := c.PostJSON(c.Endpoint("signuptokens"), createSignupTokenReq{
		User:          user,
		AllowedLogins: allowedLogins,
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

type chunkRW struct {
	c  *Client
	id string
}

func (c *chunkRW) ReadChunks(start int, end int) ([]recorder.Chunk, error) {
	out, err := c.c.Get(c.c.Endpoint("records", c.id, "chunks"), url.Values{
		"start": []string{strconv.Itoa(start)},
		"end":   []string{strconv.Itoa(end)},
	})
	if err != nil {
		return nil, err
	}
	var chunks []recorder.Chunk
	if err := json.Unmarshal(out.Bytes(), &chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (c *chunkRW) WriteChunks(chunks []recorder.Chunk) error {
	_, err := c.c.PostJSON(
		c.c.Endpoint("records", c.id, "chunks"), writeChunksReq{Chunks: chunks})
	return trace.Wrap(err)
}

func (c *chunkRW) Close() error {
	return nil
}

// TOODO(klizhentas) this should be just including appropriate service implementations
type ClientI interface {
	GetSessions() ([]session.Session, error)
	GetSession(id string) (*session.Session, error)
	CreateSession(s session.Session) error
	UpdateSession(req session.UpdateRequest) error
	UpsertParty(id string, p session.Party, ttl time.Duration) error
	UpsertCertAuthority(cert services.CertAuthority, ttl time.Duration) error
	GetCertAuthorities(caType services.CertAuthType) ([]*services.CertAuthority, error)
	DeleteCertAuthority(caType services.CertAuthID) error
	GenerateToken(role teleport.Role, ttl time.Duration) (string, error)
	RegisterUsingToken(token, hostID string, role teleport.Role) (keys PackedKeys, e error)
	RegisterNewAuthServer(domainName, token string, publicSealKey encryptor.Key) (masterKey encryptor.Key, e error)
	Log(id lunk.EventID, e lunk.Event)
	LogEntry(en lunk.Entry) error
	LogSession(sess session.Session) error
	GetEvents(filter events.Filter) ([]lunk.Entry, error)
	GetSessionEvents(filter events.Filter) ([]session.Session, error)
	GetChunkWriter(id string) (recorder.ChunkWriteCloser, error)
	GetChunkReader(id string) (recorder.ChunkReadCloser, error)
	UpsertServer(s services.Server, ttl time.Duration) error
	GetServers() ([]services.Server, error)
	GetAuthServers() ([]services.Server, error)
	UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error)
	CheckPassword(user string, password []byte, hotpToken string) error
	SignIn(user string, password []byte) (*Session, error)
	CreateWebSession(user string, prevSessionID string) (*Session, error)
	GetWebSessionInfo(user string, sid string) (*Session, error)
	GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error)
	DeleteWebSession(user string, sid string) error
	GetUsers() ([]services.User, error)
	DeleteUser(user string) error
	GenerateKeyPair(pass string) ([]byte, []byte, error)
	GenerateHostCert(key []byte, hostname, authServer string, role teleport.Role, ttl time.Duration) ([]byte, error)
	GenerateUserCert(key []byte, user string, ttl time.Duration) ([]byte, error)
	GetSignupTokenData(token string) (user string, QRImg []byte, hotpFirstValues []string, e error)
	CreateUserWithToken(token, password, hotpToken string) (*Session, error)
}
