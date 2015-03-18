// backend represents interface for accessing configuration backend for storing ACL lists and other settings
package backend

import (
	"fmt"
	"time"
)

// Backend represents configuration backend implementation for Teleport
type Backend interface {
	// UpsertUserCA upserts the user certificate authority keys in OpenSSH authorized_keys format
	UpsertUserCA(CA) error

	// GetUserCAPub returns the user certificate authority public key
	GetUserCAPub() ([]byte, error)

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
}

// CA is a set of private and public keys
type CA struct {
	Pub  []byte
	Priv []byte
}

// Server represents a running Teleport server instance
type Server struct {
	ID   string
	Addr string
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

type MissingParameterError struct {
	Param string
}

func (m *MissingParameterError) Error() string {
	return fmt.Sprintf("missing required parameter '%v'", m.Param)
}
