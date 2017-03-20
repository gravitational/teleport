package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"golang.org/x/crypto/ssh"
)

// CertParams defines all parameters needed to generate a host certificate.
type CertParams struct {
	PrivateCASigningKey []byte         // PrivateCASigningKey is the private key of the CA that will sign the public key of the host.
	PublicHostKey       []byte         // PublicHostKey is the public key of the host.
	HostID              string         // HostID is used by Teleport to uniquely identify a node within a cluster.
	NodeName            string         // NodeName is the DNS name of the node.
	ClusterName         string         // ClusterName is the name of the cluster within which a node lives.
	Roles               teleport.Roles // Roles identifies the roles of a Teleport instance.
	TTL                 time.Duration  // TTL defines how long a certificate is valid for.
}

func (c *CertParams) Check() error {
	if c.HostID == "" || c.ClusterName == "" {
		return trace.BadParameter("HostID [%q] and ClusterName [%q] are required",
			c.HostID, c.ClusterName)
	}

	if err := c.Roles.Check(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CertAuthority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthority interface {
	// Resource sets common resource properties
	Resource
	// GetID returns certificate authority ID -
	// combined type and name
	GetID() CertAuthID
	// GetType returns user or host certificate authority
	GetType() CertAuthType
	// GetClusterName returns cluster name this cert authority
	// is associated with
	GetClusterName() string
	// GetCheckingKeys returns public keys to check signature
	GetCheckingKeys() [][]byte
	// GetSigning keys returns signing keys
	GetSigningKeys() [][]byte
	// GetRoles returns a list of roles assumed by users signed by this CA
	GetRoles() []string
	// FirstSigningKey returns first signing key or returns error if it's not here
	// The first key is returned because multiple keys can exist during key rotation.
	FirstSigningKey() ([]byte, error)
	// GetRawObject returns raw object data, used for migrations
	GetRawObject() interface{}
	// Check checks object for errors
	Check() error
	// SetSigningKeys sets signing keys
	SetSigningKeys([][]byte) error
	// AddRole adds a role to ca role list
	AddRole(name string)
	// Checkers returns public keys that can be used to check cert authorities
	Checkers() ([]ssh.PublicKey, error)
	// Signers returns a list of signers that could be used to sign keys
	Signers() ([]ssh.Signer, error)
	// V1 returns V1 version of the resource
	V1() *CertAuthorityV1
	// V2 returns V2 version of the resource
	V2() *CertAuthorityV2
}

// NewCertAuthority returns new cert authority
func NewCertAuthority(caType CertAuthType, clusterName string, signingKeys, checkingKeys [][]byte, roles []string) CertAuthority {
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		Metadata: Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Roles:        roles,
			Type:         caType,
			ClusterName:  clusterName,
			CheckingKeys: checkingKeys,
			SigningKeys:  signingKeys,
		},
	}
}

// CertAuthoritiesToV1 converts list of cert authorities to V1 slice
func CertAuthoritiesToV1(in []CertAuthority) ([]CertAuthorityV1, error) {
	out := make([]CertAuthorityV1, len(in))
	type cav1 interface {
		V1() *CertAuthorityV1
	}
	for i, ca := range in {
		v1, ok := ca.(cav1)
		if !ok {
			return nil, trace.BadParameter("could not transform object to V1")
		}
		out[i] = *(v1.V1())
	}
	return out, nil
}

// CertAuthorityV2 is version 1 resource spec for Cert Authority
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

// GetMetadata returns object metadata
func (c *CertAuthorityV2) GetMetadata() Metadata {
	return c.Metadata
}

// SetExpiry sets expiry time for the object
func (c *CertAuthorityV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expires retuns object expiry setting
func (c *CertAuthorityV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (c *CertAuthorityV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// V2 returns V2 version of the resouirce - itself
func (c *CertAuthorityV2) V2() *CertAuthorityV2 {
	return c
}

// V1 returns V1 version of the object
func (c *CertAuthorityV2) V1() *CertAuthorityV1 {
	return &CertAuthorityV1{
		Type:         c.Spec.Type,
		DomainName:   c.Spec.ClusterName,
		CheckingKeys: c.Spec.CheckingKeys,
		SigningKeys:  c.Spec.SigningKeys,
	}
}

// AddRole adds a role to ca role list
func (ca *CertAuthorityV2) AddRole(name string) {
	for _, r := range ca.Spec.Roles {
		if r == name {
			return
		}
	}
	ca.Spec.Roles = append(ca.Spec.Roles, name)
}

// GetSigning keys returns signing keys
func (ca *CertAuthorityV2) GetSigningKeys() [][]byte {
	return ca.Spec.SigningKeys
}

// SetSigningKeys sets signing keys
func (ca *CertAuthorityV2) SetSigningKeys(keys [][]byte) error {
	ca.Spec.SigningKeys = keys
	return nil
}

// GetID returns certificate authority ID -
// combined type and name
func (ca *CertAuthorityV2) GetID() CertAuthID {
	return CertAuthID{Type: ca.Spec.Type, DomainName: ca.Metadata.Name}
}

// SetName sets cert authority name
func (ca *CertAuthorityV2) SetName(name string) {
	ca.Metadata.SetName(name)
}

// GetName returns cert authority name
func (ca *CertAuthorityV2) GetName() string {
	return ca.Metadata.Name
}

// GetType returns user or host certificate authority
func (ca *CertAuthorityV2) GetType() CertAuthType {
	return ca.Spec.Type
}

// GetClusterName returns cluster name this cert authority
// is associated with
func (ca *CertAuthorityV2) GetClusterName() string {
	return ca.Spec.ClusterName
}

// GetCheckingKeys returns public keys to check signature
func (ca *CertAuthorityV2) GetCheckingKeys() [][]byte {
	return ca.Spec.CheckingKeys
}

// GetRoles returns a list of roles assumed by users signed by this CA
func (ca *CertAuthorityV2) GetRoles() []string {
	return ca.Spec.Roles
}

// GetRawObject returns raw object data, used for migrations
func (ca *CertAuthorityV2) GetRawObject() interface{} {
	return ca.rawObject
}

// FirstSigningKey returns first signing key or returns error if it's not here
func (ca *CertAuthorityV2) FirstSigningKey() ([]byte, error) {
	if len(ca.Spec.SigningKeys) == 0 {
		return nil, trace.NotFound("%v has no signing keys", ca.Metadata.Name)
	}
	return ca.Spec.SigningKeys[0], nil
}

// ID returns id (consisting of domain name and type) that
// identifies the authority this key belongs to
func (ca *CertAuthorityV2) ID() *CertAuthID {
	return &CertAuthID{DomainName: ca.Spec.ClusterName, Type: ca.Spec.Type}
}

// Checkers returns public keys that can be used to check cert authorities
func (ca *CertAuthorityV2) Checkers() ([]ssh.PublicKey, error) {
	out := make([]ssh.PublicKey, 0, len(ca.Spec.CheckingKeys))
	for _, keyBytes := range ca.Spec.CheckingKeys {
		key, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, trace.BadParameter("invalid authority public key (len=%d): %v", len(keyBytes), err)
		}
		out = append(out, key)
	}
	return out, nil
}

// Signers returns a list of signers that could be used to sign keys
func (ca *CertAuthorityV2) Signers() ([]ssh.Signer, error) {
	out := make([]ssh.Signer, 0, len(ca.Spec.SigningKeys))
	for _, keyBytes := range ca.Spec.SigningKeys {
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, signer)
	}
	return out, nil
}

// Check checks if all passed parameters are valid
func (ca *CertAuthorityV2) Check() error {
	err := ca.ID().Check()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Checkers()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = ca.Signers()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CertAuthoritySpecV2 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthoritySpecV2 struct {
	// Type is either user or host certificate authority
	Type CertAuthType `json:"type"`
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
}

// CertAuthoritySpecV2Schema is JSON schema for cert authority V2
const CertAuthoritySpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["type", "cluster_name", "checking_keys"],
  "properties": {
    "type": {"type": "string"},
    "cluster_name": {"type": "string"},
    "checking_keys": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "signing_keys": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    }
  }
}`

// CertAuthorityV1 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthorityV1 struct {
	// Type is either user or host certificate authority
	Type CertAuthType `json:"type"`
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

// V1 returns V1 version of the resource
func (c *CertAuthorityV1) V1() *CertAuthorityV1 {
	return c
}

// V2 returns V2 version of the resource
func (c *CertAuthorityV1) V2() *CertAuthorityV2 {
	return &CertAuthorityV2{
		Kind:    KindCertAuthority,
		Version: V2,
		Metadata: Metadata{
			Name:      c.DomainName,
			Namespace: defaults.Namespace,
		},
		Spec: CertAuthoritySpecV2{
			Type:         c.Type,
			ClusterName:  c.DomainName,
			CheckingKeys: c.CheckingKeys,
			SigningKeys:  c.SigningKeys,
		},
		rawObject: *c,
	}
}

var certAuthorityMarshaler CertAuthorityMarshaler = &TeleportCertAuthorityMarshaler{}

// SetCertAuthorityMarshaler sets global user marshaler
func SetCertAuthorityMarshaler(u CertAuthorityMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	certAuthorityMarshaler = u
}

// GetCertAuthorityMarshaler returns currently set user marshaler
func GetCertAuthorityMarshaler() CertAuthorityMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
	return certAuthorityMarshaler
}

// CertAuthorityMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type CertAuthorityMarshaler interface {
	// UnmarshalCertAuthority unmarhsals cert authority from binary representation
	UnmarshalCertAuthority(bytes []byte) (CertAuthority, error)
	// MarshalCertAuthority to binary representation
	MarshalCertAuthority(c CertAuthority, opts ...MarshalOption) ([]byte, error)
	// GenerateCertAuthority is used to generate new cert authority
	// based on standard teleport one and is used to add custom
	// parameters and extend it in extensions of teleport
	GenerateCertAuthority(CertAuthority) (CertAuthority, error)
}

// GetCertAuthoritySchema returns JSON Schema for cert authorities
func GetCertAuthoritySchema() string {
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, CertAuthoritySpecV2Schema)
}

type TeleportCertAuthorityMarshaler struct{}

// GenerateCertAuthority is used to generate new cert authority
// based on standard teleport one and is used to add custom
// parameters and extend it in extensions of teleport
func (*TeleportCertAuthorityMarshaler) GenerateCertAuthority(ca CertAuthority) (CertAuthority, error) {
	return ca, nil
}

// UnmarshalUser unmarshals user from JSON
func (*TeleportCertAuthorityMarshaler) UnmarshalCertAuthority(bytes []byte) (CertAuthority, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var ca CertAuthorityV1
		err := json.Unmarshal(bytes, &ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ca.V2(), nil
	case V2:
		var ca CertAuthorityV2
		if err := utils.UnmarshalWithSchema(GetCertAuthoritySchema(), &ca, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		utils.UTC(&ca.Metadata.Expires)
		return &ca, nil
	}

	return nil, trace.BadParameter("cert authority resource version %v is not supported", h.Version)
}

// MarshalUser marshalls cert authority into JSON
func (*TeleportCertAuthorityMarshaler) MarshalCertAuthority(ca CertAuthority, opts ...MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type cav1 interface {
		V1() *CertAuthorityV1
	}

	type cav2 interface {
		V2() *CertAuthorityV2
	}
	version := cfg.GetVersion()
	switch version {
	case V1:
		v, ok := ca.(cav1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V1)
		}
		return json.Marshal(v.V1())
	case V2:
		v, ok := ca.(cav2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v", V2)
		}
		return json.Marshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}
