package legacy

import (
	"time"

	"github.com/gravitational/teleport/lib/services"
)

// TLSKeyPair is a TLS key pair
type TLSKeyPair struct {
	// Cert is a PEM encoded TLS cert
	Cert []byte `json:"cert,omitempty"`
	// Key is a PEM encoded TLS key
	Key []byte `json:"key,omitempty"`
}

// CertAuthorityV2 is version 2 resource spec for Cert Authority
type CertAuthorityV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains cert authority specification
	Spec CertAuthoritySpecV2 `json:"spec"`
	// rawObject is object that is raw object stored in DB
	// without any conversions applied, used in migrations
	rawObject interface{}
}

// Rotation is a status of the rotation of the certificate authority
type Rotation struct {
	// State could be one of "init" or "in_progress".
	State string `json:"state,omitempty"`
	// Phase is the current rotation phase.
	Phase string `json:"phase,omitempty"`
	// Mode sets manual or automatic rotation mode.
	Mode string `json:"mode,omitempty"`
	// CurrentID is the ID of the rotation operation
	// to differentiate between rotation attempts.
	CurrentID string `json:"current_id"`
	// Started is set to the time when rotation has been started
	// in case if the state of the rotation is "in_progress".
	Started time.Time `json:"started,omitempty"`
	// GracePeriod is a period during which old and new CA
	// are valid for checking purposes, but only new CA is issuing certificates.
	GracePeriod Duration `json:"grace_period,omitempty"`
	// LastRotated specifies the last time of the completed rotation.
	LastRotated time.Time `json:"last_rotated,omitempty"`
	// Schedule is a rotation schedule - used in
	// automatic mode to switch beetween phases.
	Schedule RotationSchedule `json:"schedule,omitempty"`
}

// RotationSchedule is a rotation schedule setting time switches
// for different phases.
type RotationSchedule struct {
	// UpdateClients specifies time to switch to the "Update clients" phase
	UpdateClients time.Time `json:"update_clients,omitempty"`
	// UpdateServers specifies time to switch to the "Update servers" phase.
	UpdateServers time.Time `json:"update_servers,omitempty"`
	// Standby specifies time to switch to the "Standby" phase.
	Standby time.Time `json:"standby,omitempty"`
}

// CertAuthoritySpecV2 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthoritySpecV2 struct {
	// Type is either user or host certificate authority
	Type services.CertAuthType `json:"type"`
	// DELETE IN(2.7.0) this field is deprecated,
	// as resource name matches cluster name after migrations.
	// and this property is enforced by the auth server code.
	// ClusterName identifies cluster name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	ClusterName string `json:"cluster_name"`
	// Checkers is a list of SSH public keys that can be used to check
	// certificate signatures
	CheckingKeys [][]byte `json:"checking_keys"`
	// SigningKeys is a list of private keys used for signing
	SigningKeys [][]byte `json:"signing_keys,omitempty"`
	// Roles is a list of roles assumed by users signed by this CA
	Roles []string `json:"roles,omitempty"`
	// RoleMap specifies role mappings to remote roles
	RoleMap RoleMap `json:"role_map,omitempty"`
	// TLS is a list of TLS key pairs
	TLSKeyPairs []TLSKeyPair `json:"tls_key_pairs,omitempty"`
	// Rotation is a status of the certificate authority rotation
	Rotation *Rotation `json:"rotation,omitempty"`
}

// CertAuthorityV1 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthorityV1 struct {
	// Type is either user or host certificate authority
	Type services.CertAuthType `json:"type"`
	// DomainName identifies domain name this authority serves,
	// for host authorities that means base hostname of all servers,
	// for user authorities that means organization name
	DomainName string `json:"domain_name"`
	// Checkers is a list of SSH public keys that can be used to check
	// certificate signatures
	CheckingKeys [][]byte `json:"checking_keys"`
	// SigningKeys is a list of private keys used for signing
	SigningKeys [][]byte `json:"signing_keys"`
	// AllowedLogins is a list of allowed logins for users within
	// this certificate authority
	AllowedLogins []string `json:"allowed_logins"`
}
