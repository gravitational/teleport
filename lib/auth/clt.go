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
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/codahale/lunk"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

const CurrentVersion = "v1"

// Certificate authority endpoints control user and host Certificate Authorities.
// They are central mechanism for authenticating users and hosts within
// the cluster.
//
// Client is HTTP API client that connects to the remote server
type Client struct {
	roundtrip.Client
}

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

func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion, params...)
	if err != nil {
		return nil, err
	}
	return &Client{*c}, nil
}

func (c *Client) convertResponse(
	re *roundtrip.Response, err error) (*roundtrip.Response, error) {

	if err != nil {
		return nil, err
	}
	if re.Code() == http.StatusNotFound {
		return nil, &teleport.NotFoundError{Message: string(re.Bytes())}
	}
	if re.Code() < 200 || re.Code() > 299 {
		return nil, fmt.Errorf("error: %v", string(re.Bytes()))
	}
	return re, nil
}

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(
	endpoint string,
	vals url.Values,
	files ...roundtrip.File) (*roundtrip.Response, error) {

	return c.convertResponse(c.Client.PostForm(endpoint, vals, files...))
}

// Get issues http GET request to the server
func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return c.convertResponse(c.Client.Get(u, params))
}

// Delete issues http Delete Request to the server
func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return c.convertResponse(c.Client.Delete(u))
}

func (c *Client) GetSessions() ([]session.Session, error) {
	out, err := c.Get(c.Endpoint("sessions"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *sessionsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Sessions, nil
}

func (c *Client) GetSession(id string) (*session.Session, error) {
	out, err := c.Get(c.Endpoint("sessions", id), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *sessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return &re.Session, nil
}

func (c *Client) DeleteSession(id string) error {
	_, err := c.Delete(c.Endpoint("sessions", id))
	return err
}

func (c *Client) UpsertSession(id string, ttl time.Duration) error {
	out, err := c.PostForm(c.Endpoint("sessions"), url.Values{
		"id":  []string{id},
		"ttl": []string{ttl.String()},
	})
	if err != nil {
		return err
	}
	var re *sessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return err
	}
	return nil
}

func (c *Client) UpsertParty(id string, p session.Party, ttl time.Duration) error {
	a, err := p.LastActive.MarshalText()
	if err != nil {
		return err
	}
	out, err := c.PostForm(c.Endpoint("sessions", id, "parties"), url.Values{
		"id":          []string{p.ID},
		"site":        []string{p.Site},
		"user":        []string{p.User},
		"server_addr": []string{p.ServerAddr},
		"ttl":         []string{ttl.String()},
		"last_active": []string{string(a)},
	})
	if err != nil {
		return err
	}
	var re *partyResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return err
	}
	return nil
}

func (c *Client) UpsertRemoteCertificate(cert services.CertificateAuthority, ttl time.Duration) error {
	out, err := c.PostForm(c.Endpoint("ca", "remote", cert.Type, "hosts", cert.DomainName), url.Values{
		"key": []string{string(cert.PublicKey)},
		"ttl": []string{ttl.String()},
		"id":  []string{cert.ID},
	})
	if err != nil {
		return err
	}
	var re *remoteCertResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return err
	}
	return nil
}

func (c *Client) GetRemoteCertificates(ctype string, domainName string) ([]services.CertificateAuthority, error) {
	out, err := c.Get(c.Endpoint("ca", "remote", ctype), url.Values{
		"domain": []string{domainName},
	})
	if err != nil {
		return nil, err
	}
	var re *remoteCertsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.RemoteCertificates, nil
}

func (c *Client) GetTrustedCertificates(certType string) ([]services.CertificateAuthority, error) {
	out, err := c.Get(c.Endpoint("ca", "trusted", certType), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *remoteCertsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.RemoteCertificates, nil
}

func (c *Client) DeleteRemoteCertificate(ctype string, domainName, id string) error {
	_, err := c.Delete(c.Endpoint("ca", "remote", ctype, "hosts", domainName, id))
	return err
}

// GenerateToken creates a special provisioning token for the SSH server
// with the specified hostname that is valid for ttl period seconds.
//
// This token is used by SSH server to authenticate with Auth server
// and get signed certificate and private key from the auth server.
//
// The token can be used only once and only to generate the hostname
// specified in it.
func (c *Client) GenerateToken(nodename, role string, ttl time.Duration) (string, error) {
	out, err := c.PostForm(c.Endpoint("tokens"), url.Values{
		"domain": []string{nodename},
		"role":   []string{role},
		"ttl":    []string{ttl.String()},
	})
	if err != nil {
		return "", err
	}
	var re *tokenResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.Token, nil
}

func (c *Client) RegisterUsingToken(token, nodename, role string) (PackedKeys, error) {
	out, err := c.PostForm(c.Endpoint("tokens", "register"), url.Values{
		"token":  []string{token},
		"domain": []string{nodename},
		"role":   []string{role},
	})
	if err != nil {
		return PackedKeys{}, err
	}
	var keys PackedKeys
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return PackedKeys{}, err
	}
	return keys, nil
}

func (c *Client) RegisterNewAuthServer(nodename, token string,
	publicSealKey encryptor.Key) (masterKey encryptor.Key, e error) {

	pkeyJSON, err := json.Marshal(publicSealKey)
	if err != nil {
		return encryptor.Key{}, err
	}
	out, err := c.PostForm(c.Endpoint("tokens", "register", "auth"), url.Values{
		"token":  []string{token},
		"domain": []string{nodename},
		"key":    []string{string(pkeyJSON)},
	})
	if err != nil {
		return encryptor.Key{}, err
	}
	if err := json.Unmarshal(out.Bytes(), &masterKey); err != nil {
		return encryptor.Key{}, err
	}
	return masterKey, nil
}

func (c *Client) Log(id lunk.EventID, e lunk.Event) {
	en := lunk.NewEntry(id, e)
	en.Time = time.Now()
	c.LogEntry(en)
}

func (c *Client) LogEntry(en lunk.Entry) error {
	bt, err := json.Marshal(en)
	if err != nil {
		return err
	}
	file := roundtrip.File{
		Name:     "event",
		Filename: "event.json",
		Reader:   bytes.NewReader(bt),
	}
	_, err = c.PostForm(c.Endpoint("events"), url.Values{}, file)
	return err
}

func (c *Client) GetEvents(filter events.Filter) ([]lunk.Entry, error) {
	vals, err := events.FilterToURL(filter)
	if err != nil {
		return nil, err
	}
	out, err := c.Get(c.Endpoint("events"), vals)
	if err != nil {
		return nil, err
	}
	var re *eventsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Events, nil
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
	args := upsertServerArgs{
		Server: s,
		TTL:    ttl,
	}
	_, err := c.PostJSON(c.Endpoint("servers"), args)
	return err
}

// GetServers returns the list of servers registered in the cluster.
func (c *Client) GetServers() ([]services.Server, error) {
	out, err := c.Get(c.Endpoint("servers"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *serversResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Servers, nil
}

// UpsertWebTun creates a persistent SSH tunnel to the specified web target
// server that is valid for ttl period.
// See services.WebTun documentation for details
func (c *Client) UpsertWebTun(wt services.WebTun, ttl time.Duration) error {
	_, err := c.PostForm(c.Endpoint("tunnels", "web"), url.Values{
		"target": []string{string(wt.TargetAddr)},
		"proxy":  []string{string(wt.ProxyAddr)},
		"prefix": []string{string(wt.Prefix)},
		"ttl":    []string{ttl.String()},
	})
	return err
}

// GetWebTuns returns a list of web tunnels supported by the system
func (c *Client) GetWebTuns() ([]services.WebTun, error) {
	out, err := c.Get(c.Endpoint("tunnels", "web"), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *webTunsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Tunnels, nil
}

// GetWebTun retruns the web tunel details by it unique prefix
func (c *Client) GetWebTun(prefix string) (*services.WebTun, error) {
	out, err := c.Get(c.Endpoint("tunnels", "web", prefix), url.Values{})
	if err != nil {
		return nil, err
	}
	var re *webTunResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return &re.Tunnel, nil
}

// DeleteWebTun deletes the tunnel by prefix
func (c *Client) DeleteWebTun(prefix string) error {
	_, err := c.Delete(c.Endpoint("tunnels", "web", prefix))
	return err
}

// UpsertPassword updates web access password for the user
func (c *Client) UpsertPassword(user string,
	password []byte) (hotpURL string, hotpQR []byte, err error) {
	out, err := c.PostForm(
		c.Endpoint("users", user, "web", "password"),
		url.Values{"password": []string{string(password)}},
	)
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
	_, err := c.PostForm(
		c.Endpoint("users", user, "web", "password", "check"),
		url.Values{
			"password":  []string{string(password)},
			"hotpToken": []string{hotpToken},
		})
	return err
}

// SignIn checks if the web access password is valid, and if it is valid
// returns a secure web session id.
func (c *Client) SignIn(user string, password []byte) (string, error) {
	out, err := c.PostForm(
		c.Endpoint("users", user, "web", "signin"),
		url.Values{
			"password": []string{string(password)},
		},
	)
	if err != nil {
		return "", err
	}
	var re *webSessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.SID, nil
}

// GetWebSession check if a web sesion is valid, returns session id in case if
// it is valid, or error otherwise.
func (c *Client) GetWebSession(user string, sid string) (string, error) {
	out, err := c.Get(
		c.Endpoint("users", user, "web", "sessions", sid), url.Values{})
	if err != nil {
		return "", err
	}
	var re *webSessionResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return "", err
	}
	return re.SID, nil
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
	var re *webSessionsResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Keys, nil
}

// DeleteWebSession deletes a web session for this user by id
func (c *Client) DeleteWebSession(user string, sid string) error {
	_, err := c.Delete(c.Endpoint("users", user, "web", "sessions", sid))
	return err
}

// GetUsers returns a list of usernames registered in the system
func (c *Client) GetUsers() ([]services.User, error) {
	out, err := c.Get(c.Endpoint("users"), url.Values{})
	if err != nil {
		return nil, err
	}
	var users *usersResponse
	if err := json.Unmarshal(out.Bytes(), &users); err != nil {
		return nil, err
	}
	return users.Users, nil
}

// DeleteUser deletes a user by username
func (c *Client) DeleteUser(user string) error {
	_, err := c.Delete(c.Endpoint("users", user))
	return err
}

// UpsertUserKey takes public key of the user, generates certificate for it
// and adds it to the authorized keys database. It returns certificate signed
// by user Certificate Authority in case of success, error otherwise. The certificate will be
// valid for the duration of the ttl passed in.
func (c *Client) UpsertUserKey(username string,
	key services.AuthorizedKey, ttl time.Duration) ([]byte, error) {

	out, err := c.PostForm(c.Endpoint("users", username, "keys"), url.Values{
		"key": []string{string(key.Value)},
		"id":  []string{key.ID},
		"ttl": []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

// GetUserKeys returns a list of keys registered for this user.
// This list does not include the temporary keys associated with user
// web sessions.
func (c *Client) GetUserKeys(user string) ([]services.AuthorizedKey, error) {
	out, err := c.Get(c.Endpoint("users", user, "keys"), url.Values{})
	if err != nil {
		return nil, err
	}
	var keys *pubKeysResponse
	if err := json.Unmarshal(out.Bytes(), &keys); err != nil {
		return nil, err
	}
	return keys.PubKeys, nil
}

// DeleteUserKey deletes a key by id for a given user
func (c *Client) DeleteUserKey(username string, id string) error {
	_, err := c.Delete(c.Endpoint("users", username, "keys", id))
	return err
}

// Returns host certificate authority public key. This public key is used to
// validate if host certificates were signed by the proper key.
func (c *Client) GetHostCertificateAuthority() (*services.CertificateAuthority, error) {
	out, err := c.Get(c.Endpoint("ca", "host", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubCert services.CertificateAuthority
	if err := json.Unmarshal(out.Bytes(), &pubCert); err != nil {
		return nil, err
	}
	return &pubCert, err
}

// Returns user certificate authority public key.
// This public key is used to check if the users certificate is valid and was
// signed by this  authority.
func (c *Client) GetUserCertificateAuthority() (*services.CertificateAuthority, error) {
	out, err := c.Get(c.Endpoint("ca", "user", "keys", "pub"), url.Values{})
	if err != nil {
		return nil, err
	}
	var pubCert services.CertificateAuthority
	if err := json.Unmarshal(out.Bytes(), &pubCert); err != nil {
		return nil, err
	}
	return &pubCert, err
}

// GenerateKeyPair generates SSH private/public key pair optionally protected
// by password. If the pass parameter is an empty string, the key pair
// is not password-protected.
func (c *Client) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	out, err := c.PostForm(c.Endpoint("keypair"), url.Values{})
	if err != nil {
		return nil, nil, err
	}
	var kp *keyPairResponse
	if err := json.Unmarshal(out.Bytes(), &kp); err != nil {
		return nil, nil, err
	}
	return kp.PrivKey, []byte(kp.PubKey), err
}

// GenerateHostCert takes the public key in the Open SSH ``authorized_keys``
// plain text format, signs it using Host Certificate Authority private key and returns the
// resulting certificate.
func (c *Client) GenerateHostCert(
	key []byte, id, hostname, role string, ttl time.Duration) ([]byte, error) {

	out, err := c.PostForm(c.Endpoint("ca", "host", "certs"), url.Values{
		"key":      []string{string(key)},
		"id":       []string{id},
		"hostname": []string{hostname},
		"role":     []string{role},
		"ttl":      []string{ttl.String()},
	})
	if err != nil {
		return nil, err
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}
	return []byte(cert.Cert), err
}

// GenerateUserCert takes the public key in the Open SSH ``authorized_keys``
// plain text format, signs it using User Certificate Authority signing key and returns the
// resulting certificate.
func (c *Client) GenerateUserCert(
	key []byte, id, user string, ttl time.Duration) ([]byte, error) {

	out, err := c.PostForm(c.Endpoint("ca", "user", "certs"), url.Values{
		"key":  []string{string(key)},
		"id":   []string{id},
		"user": []string{user},
		"ttl":  []string{ttl.String()},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cert *certResponse
	if err := json.Unmarshal(out.Bytes(), &cert); err != nil {
		return nil, err
	}

	return []byte(cert.Cert), err
}

// Regenerates host certificate authority private key.
// Host authority certificate is used to sign SSH hosts public keys,
// so users and auth servers can authenicate SSH servers
// when connecting to them.

// All host certificate keys will have to be regenerated and all SSH nodes will
// have to be re-provisioned after calling this method.
func (c *Client) ResetHostCertificateAuthority() error {
	_, err := c.PostForm(c.Endpoint("ca", "host", "keys"), url.Values{})
	return err
}

// Regenerates user certificate authority private key.
// User authority certificate is used to sign User SSH public keys,
// so auth server can check if that is a valid key before even hitting
// the database.
//
// All user certificates will have to be regenerated.
//
func (c *Client) ResetUserCertificateAuthority() error {
	_, err := c.PostForm(c.Endpoint("ca", "user", "keys"), url.Values{})
	return err
}

// GetBackendKeys returns IDs of all the backend encrypting keys that
// this server has
func (c *Client) GetSealKeys() ([]encryptor.Key, error) {
	out, err := c.Get(c.Endpoint("backend", "keys"), url.Values{})
	if err != nil {
		return nil, err
	}
	var res sealKeysResponse
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		return nil, err
	}
	return res.Keys, err
}

// GenerateBackendKey generates a new backend encrypting key with the
// given id and then backend makes a copy of all the data using the
// generated key for encryption
func (c *Client) GenerateSealKey(keyName string) (encryptor.Key, error) {
	out, err := c.PostForm(c.Endpoint("backend", "generatekey"), url.Values{
		"name": []string{keyName}})
	if err != nil {
		return encryptor.Key{}, err
	}

	var res sealKeyResponse
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		return encryptor.Key{}, err
	}
	return res.Key, err
}

// DeleteBackendKey deletes the backend encrypting key and all the data
// encrypted with the key
func (c *Client) DeleteSealKey(keyID string) error {
	_, err := c.Delete(c.Endpoint("backend", "keys", keyID))
	if err != nil {
		return err
	}
	return err
}

// AddBackendKey adds the given encrypting key. If backend works not in
// readonly mode, backend makes a copy of the data using the key for
// encryption
func (c *Client) AddSealKey(key encryptor.Key) error {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return err
	}
	_, err = c.PostForm(c.Endpoint("backend", "keys"), url.Values{
		"key": []string{string(keyJSON)}})
	return err
}

// GetBackendKeys returns the backend encrypting key.
func (c *Client) GetSealKey(keyID string) (encryptor.Key, error) {
	out, err := c.Get(c.Endpoint("backend", "keys", keyID), url.Values{})
	if err != nil {
		return encryptor.Key{}, err
	}
	var key sealKeyResponse
	if err := json.Unmarshal(out.Bytes(), &key); err != nil {
		return encryptor.Key{}, err
	}
	return key.Key, nil
}

// CreateSignupToken creates one time token for creating account for the user
// For each token it creates username and hotp generator
func (c *Client) CreateSignupToken(user string, mappings []string) (token string, e error) {
	if len(mappings) == 0 {
		return "", trace.Errorf("cannot create a new account without any user mappings")
	}
	out, err := c.PostForm(c.Endpoint("signuptokens"), url.Values{
		"user":     []string{user},
		"mappings": mappings,
	})
	if err != nil {
		return "", err
	}
	var result map[string]string
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", err
	}
	return result["message"], err
}

// GetSignupTokenData returns token data for a valid token
func (c *Client) GetSignupTokenData(token string) (user string,
	QRImg []byte, hotpFirstValues []string, e error) {

	out, err := c.Get(c.Endpoint("signuptokens", token), url.Values{})
	if err != nil {
		return "", nil, nil, err
	}
	var tokenData userTokenDataResponse
	if err := json.Unmarshal(out.Bytes(), &tokenData); err != nil {
		return "", nil, nil, err
	}
	return tokenData.User, tokenData.QRImg, tokenData.HotpFirstValues, nil
}

// CreateUserWithToken creates account with provided token and password.
// Account username and hotp generator are taken from token data.
// Deletes token after account creation.
func (c *Client) CreateUserWithToken(token, password, hotpToken string) error {
	_, err := c.PostForm(c.Endpoint("signuptokens", "users"), url.Values{
		"token":     []string{token},
		"password":  []string{password},
		"hotptoken": []string{hotpToken},
	})

	return err
}

func (c *Client) GetCertificateID(certType string, key ssh.PublicKey) (ID string, found bool, e error) {
	keyBytes := ssh.MarshalAuthorizedKey(key)
	out, err := c.Get(c.Endpoint("ca", "id", certType), url.Values{
		"key": []string{string(keyBytes)},
	})
	if err != nil {
		return "", false, err
	}

	var result getCertificateIDResponse
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", false, err
	}
	return result.ID, result.Found, nil
}

func (c *Client) UpsertUserMapping(certificateID, teleportUser, osUser string, ttl time.Duration) error {
	_, err := c.PostForm(c.Endpoint("usermappings"), url.Values{
		"certificateID": []string{certificateID},
		"teleportUser":  []string{teleportUser},
		"osUser":        []string{osUser},
		"ttl":           []string{ttl.String()},
	})
	return err

}

func (c *Client) DeleteUserMapping(certificateID, teleportUser, osUser string) error {
	id, err := services.UserMappingID(certificateID, teleportUser, osUser)
	if err != nil {
		return err
	}

	_, err = c.Delete(c.Endpoint("usermappings", id))
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) UserMappingExists(certificateID, teleportUser, osUser string) (bool, error) {
	id, err := services.UserMappingID(certificateID, teleportUser, osUser)
	if err != nil {
		return false, err
	}

	out, err := c.Get(c.Endpoint("usermappings", id), url.Values{})
	if err != nil {
		return false, err
	}

	var result userMappingExistsResponse
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return false, err
	}
	return result.Exists, nil

}

func (c *Client) GetAllUserMappings() (IDs []string, e error) {
	out, err := c.Get(c.Endpoint("usermappings"), url.Values{})
	if err != nil {
		return nil, err
	}
	var response getAllUserMappingsResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, err
	}
	return response.IDs, nil
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
	var re *chunksResponse
	if err := json.Unmarshal(out.Bytes(), &re); err != nil {
		return nil, err
	}
	return re.Chunks, nil
}

func (c *chunkRW) WriteChunks(chs []recorder.Chunk) error {
	files := make([]roundtrip.File, len(chs))

	for i, ch := range chs {
		bt, err := json.Marshal(ch)
		if err != nil {
			return err
		}
		files[i] = roundtrip.File{
			Name:     "chunk",
			Filename: "chunk.json",
			Reader:   bytes.NewReader(bt),
		}
	}
	_, err := c.c.PostForm(
		c.c.Endpoint("records", c.id, "chunks"), url.Values{}, files...)
	return err
}

func (c *chunkRW) Close() error {
	return nil
}

// TOODO(klizhentas) this should be just including appropriate service implementations
type ClientI interface {
	GetSessions() ([]session.Session, error)
	GetSession(id string) (*session.Session, error)
	DeleteSession(id string) error
	UpsertSession(id string, ttl time.Duration) error
	UpsertParty(id string, p session.Party, ttl time.Duration) error
	UpsertRemoteCertificate(cert services.CertificateAuthority, ttl time.Duration) error
	GetRemoteCertificates(ctype string, domainName string) ([]services.CertificateAuthority, error)
	DeleteRemoteCertificate(ctype string, domainName, id string) error
	GetTrustedCertificates(certType string) ([]services.CertificateAuthority, error)
	GenerateToken(domainName, role string, ttl time.Duration) (string, error)
	RegisterUsingToken(token, domainName, role string) (keys PackedKeys, e error)
	RegisterNewAuthServer(domainName, token string, publicSealKey encryptor.Key) (masterKey encryptor.Key, e error)
	Log(id lunk.EventID, e lunk.Event)
	LogEntry(en lunk.Entry) error
	GetEvents(filter events.Filter) ([]lunk.Entry, error)
	GetChunkWriter(id string) (recorder.ChunkWriteCloser, error)
	GetChunkReader(id string) (recorder.ChunkReadCloser, error)
	UpsertServer(s services.Server, ttl time.Duration) error
	GetServers() ([]services.Server, error)
	UpsertWebTun(wt services.WebTun, ttl time.Duration) error
	GetWebTuns() ([]services.WebTun, error)
	GetWebTun(prefix string) (*services.WebTun, error)
	DeleteWebTun(prefix string) error
	UpsertPassword(user string, password []byte) (hotpURL string, hotpQR []byte, err error)
	CheckPassword(user string, password []byte, hotpToken string) error
	SignIn(user string, password []byte) (string, error)
	GetWebSession(user string, sid string) (string, error)
	GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error)
	DeleteWebSession(user string, sid string) error
	GetUsers() ([]services.User, error)
	DeleteUser(user string) error
	GetHostCertificateAuthority() (*services.CertificateAuthority, error)
	GetUserCertificateAuthority() (*services.CertificateAuthority, error)
	GenerateKeyPair(pass string) ([]byte, []byte, error)
	GenerateHostCert(key []byte, id, hostname, role string, ttl time.Duration) ([]byte, error)
	GenerateUserCert(key []byte, id, user string, ttl time.Duration) ([]byte, error)
	ResetHostCertificateAuthority() error
	ResetUserCertificateAuthority() error
	GetSignupTokenData(token string) (user string, QRImg []byte, hotpFirstValues []string, e error)
	CreateUserWithToken(token, password, hotpToken string) error
}
