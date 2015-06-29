// backend represents interface for accessing configuration backend for storing ACL lists and other settings
package backend

import (
	"fmt"
	"net/url"
	"time"
)

// TODO(klizhentas) this is bloated. Split it into little backend interfaces
// Backend represents configuration backend implementation for Teleport
type Backend interface {
	// Grab a lock that will be released automatically in ttl time
	AcquireLock(token string, ttl time.Duration) error

	// Grab a lock that will be released automatically in ttl time
	ReleaseLock(token string) error

	// UpsertUserCA upserts the user certificate authority keys in OpenSSH authorized_keys format
	UpsertUserCA(CA) error

	// GetUserCAPub returns the user certificate authority public key
	GetUserCAPub() ([]byte, error)

	// Remote Certificate management
	UpsertRemoteCert(RemoteCert, time.Duration) error
	GetRemoteCerts(ctype string, fqdn string) ([]RemoteCert, error)
	DeleteRemoteCert(ctype string, fqdn, id string) error

	// GetCA returns private, public key and certificate for user CA
	GetUserCA() (*CA, error)

	// UpsertHostCA upserts host certificate authority keys in OpenSSH authorized_keys format
	UpsertHostCA(CA) error

	// GetHostCA returns private, public key and certificate for host CA
	GetHostCA() (*CA, error)

	// GetHostCACert returns the host certificate authority certificate
	GetHostCAPub() ([]byte, error)

	// GetUserKeys returns a list of authorized keys for a given user
	// in a OpenSSH key authorized_keys format
	GetUserKeys(user string) ([]AuthorizedKey, error)

	// GetUsers  returns a list of users registered in the backend
	GetUsers() ([]string, error)

	// DeleteUser deletes a user with all the keys from the backend
	DeleteUser(user string) error

	// Upsert Public key in OpenSSH authorized Key format
	// user is a user name, keyID is a unique IDentifier for the key
	// in case if ttl is 0, the key will be upserted permanently, otherwise
	// it will expire in ttl seconds
	UpsertUserKey(user string, key AuthorizedKey, ttl time.Duration) error

	// DeleteUserKey deletes user key by given ID
	DeleteUserKey(user, key string) error

	// GetServers returns a list of registered servers
	GetServers() ([]Server, error)

	// UpsertServer registers server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertServer(s Server, ttl time.Duration) error

	// UpsertPasswordHash upserts user password hash
	UpsertPasswordHash(user string, hash []byte) error

	// GetPasswordHash returns the password hash for a given user
	GetPasswordHash(user string) ([]byte, error)

	// UpsertSession
	UpsertWebSession(user, sid string, s WebSession, ttl time.Duration) error

	// GetWebSession
	GetWebSession(user, sid string) (*WebSession, error)

	// GetWebSessionsKeys
	GetWebSessionsKeys(user string) ([]AuthorizedKey, error)

	// DeleteWebSession
	DeleteWebSession(user, sid string) error

	UpsertWebTun(WebTun, time.Duration) error

	DeleteWebTun(prefix string) error

	GetWebTun(prefix string) (*WebTun, error)

	GetWebTuns() ([]WebTun, error)

	// Tokens are provisioning tokens for the auth server
	UpsertToken(token, fqdn string, ttl time.Duration) error
	GetToken(token string) (string, error)
	DeleteToken(token string) error
}

// WebTun is a web tunnel, the SSH tunnel
// created by the SSH server to a remote web server
type WebTun struct {
	// Prefix is a domain prefix that will be used
	// to serve this tunnel
	Prefix string `json:"prefix"`
	// ProxyAddr is the address of the SSH server
	// that will be acting as a SSH proxy
	ProxyAddr string `json:"proxy"`
	// TargetAddr is the target http address of the server
	TargetAddr string `json:"target"`
}

func NewWebTun(prefix, proxyAddr, targetAddr string) (*WebTun, error) {
	if prefix == "" {
		return nil, &MissingParameterError{Param: "prefix"}
	}
	if targetAddr == "" {
		return nil, &MissingParameterError{Param: "target"}
	}
	if proxyAddr == "" {
		return nil, &MissingParameterError{Param: "proxy"}
	}
	if _, err := url.ParseRequestURI(targetAddr); err != nil {
		return nil, &BadParameterError{Param: "target", Err: err.Error()}
	}
	return &WebTun{Prefix: prefix, ProxyAddr: proxyAddr, TargetAddr: targetAddr}, nil
}

// WebSession
type WebSession struct {
	Pub  []byte `json:"pub"`
	Priv []byte `json:"priv"`
}

// CA is a set of private and public keys
type CA struct {
	Pub  []byte `json:"pub"`
	Priv []byte `json:"priv"`
}

// Server represents a running Teleport server instance
type Server struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
}

// AuthorizedKey is a key in form of OpenSSH authorized keys
type AuthorizedKey struct {
	ID    string `json:"id"`
	Value []byte `json:"value"`
}

type NotFoundError struct {
	Message string
}

func (n *NotFoundError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object not found"
	}
}

type AlreadyExistsError struct {
	Message string
}

func (n *AlreadyExistsError) Error() string {
	if n.Message != "" {
		return n.Message
	} else {
		return "Object already exists"
	}
}

type MissingParameterError struct {
	Param string
}

func (m *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter '%v'", m.Param)
}

type BadParameterError struct {
	Param string
	Err   string
}

func (m *BadParameterError) Error() string {
	return fmt.Sprintf("bad parameter '%v', %v", m.Param, m.Err)
}

type RemoteCert struct {
	Type  string
	ID    string
	FQDN  string
	Value []byte
}

const (
	HostCert = "host"
	UserCert = "user"
)
