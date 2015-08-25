package auth

import (
	"time"

	"github.com/gravitational/teleport/backend"
	"github.com/gravitational/teleport/services"
)

// AccessPoint is a interface needed by nodes to control the access
// to the node, and provide heartbeats
type AccessPoint interface {
	// GetServers returns a list of registered servers
	GetServers() ([]services.Server, error)

	// UpsertServer registers server presence, permanently if ttl is 0 or
	// for the specified duration with second resolution if it's >= 1 second
	UpsertServer(s services.Server, ttl time.Duration) error

	// GetUserCAPub returns the user certificate authority public key
	GetUserCAPub() ([]byte, error)

	// GetUserKeys returns a list of authorized keys for a given user
	// in a OpenSSH key authorized_keys format
	GetUserKeys(user string) ([]services.AuthorizedKey, error)

	// GetWebSessionsKeys returns a list of generated public keys
	// associated with user web session
	GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error)

	// GetRemoteCerts returns a list of trusted remote certificates
	GetRemoteCerts(ctype, fqdn string) ([]services.RemoteCert, error)
}

type BackendAccessPoint struct {
	caS           *services.CAService
	presenceS     *services.PresenceService
	provisioningS *services.ProvisioningService
	userS         *services.UserService
	webS          *services.WebService
}

func NewBackendAccessPoint(bk backend.Backend) *BackendAccessPoint {
	ap := BackendAccessPoint{}
	ap.caS = services.NewCAService(bk)
	ap.presenceS = services.NewPresenceService(bk)
	ap.provisioningS = services.NewProvisioningService(bk)
	ap.userS = services.NewUserService(bk)
	ap.webS = services.NewWebService(bk)

	return &ap
}

// GetServers returns a list of registered servers
func (ap *BackendAccessPoint) GetServers() ([]services.Server, error) {
	return ap.presenceS.GetServers()
}

// UpsertServer registers server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (ap *BackendAccessPoint) UpsertServer(s services.Server, ttl time.Duration) error {
	return ap.presenceS.UpsertServer(s, ttl)
}

// GetUserCAPub returns the user certificate authority public key
func (ap *BackendAccessPoint) GetUserCAPub() ([]byte, error) {
	return ap.caS.GetUserCAPub()
}

// GetUserKeys returns a list of authorized keys for a given user
// in a OpenSSH key authorized_keys format
func (ap *BackendAccessPoint) GetUserKeys(user string) ([]services.AuthorizedKey, error) {
	return ap.userS.GetUserKeys(user)
}

// GetWebSessionsKeys returns a list of generated public keys
// associated with user web session
func (ap *BackendAccessPoint) GetWebSessionsKeys(user string) ([]services.AuthorizedKey, error) {
	return ap.webS.GetWebSessionsKeys(user)
}

// GetRemoteCerts returns a list of trusted remote certificates
func (ap *BackendAccessPoint) GetRemoteCerts(ctype, fqdn string) ([]services.RemoteCert, error) {
	return ap.caS.GetRemoteCerts(ctype, fqdn)
}
