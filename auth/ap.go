package auth

import (
	"time"

	"github.com/gravitational/teleport/backend"
)

// AccessPoint is a interface needed by nodes to control the access
// to the node, and provide heartbeats
type AccessPoint interface {
	// GetServers returns a list of registered servers
	GetServers() ([]backend.Server, error)

	// UpsertServer registers server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertServer(s backend.Server, ttl time.Duration) error

	// GetUserCAPub returns the user certificate authority public key
	GetUserCAPub() ([]byte, error)

	// GetUserKeys returns a list of authorized keys for a given user
	// in a OpenSSH key authorized_keys format
	GetUserKeys(user string) ([]backend.AuthorizedKey, error)

	// GetWebSessionsKeys returns a list of generated public keys
	// associated with user web session
	GetWebSessionsKeys(user string) ([]backend.AuthorizedKey, error)

	// GetRemoteCerts returns a list of trusted remote certificates
	GetRemoteCerts(ctype, fqdn string) ([]backend.RemoteCert, error)
}
