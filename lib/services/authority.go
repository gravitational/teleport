package services

import (
	"encoding/json"
	"fmt"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// CertAuthority is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthority interface {
	// GetName returns cert authority name
	GetName() string
	// GetType returns user or host certificate authority
	GetType() string
	// GetClusterName returns cluster name this cert authority
	// is associated with
	GetClusterName() string
	// GetCheckingKeys returns public keys to check signature
	GetCheckingKeys() [][]byte
	// GetRoles returns a list of roles assumed by users signed by this CA
	GetRoles()
	// FirstSigningKey returns first signing key or returns error if it's not here
	FirstSigningKey() ([]byte, error)
	// GetRawObject returns raw object data, used for migrations
	GetRawObject() interface{}
}

// CertAuthorityV1 is version 1 resource spec for Cert Authority
type CertAuthorityV1 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is connector metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains cert authority specification
	Spec CertAuthoritySpecV1 `json:"spec"`
	// rawObject is object that is raw object stored in DB
	// without any migrations applied, used in migrations
	rawObject interface{}
}

// FirstSigningKey returns first signing key or returns error if it's not here
func (ca *CertAuthorityV1) FirstSigningKey() ([]byte, error) {
	if len(ca.Spec.SigningKeys) == 0 {
		return nil, trace.NotFound("%v has no signing keys", ca.Metadata.Name)
	}
	return ca.Spec.SigningKeys[0], nil
}

// ID returns id (consisting of domain name and type) that
// identifies the authority this key belongs to
func (ca *CertAuthorityV1) ID() *CertAuthID {
	return &CertAuthID{DomainName: ca.Spec.ClusterName, Type: ca.Spec.Type}
}

// Checkers returns public keys that can be used to check cert authorities
func (ca *CertAuthorityV1) Checkers() ([]ssh.PublicKey, error) {
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
func (ca *CertAuthorityV1) Signers() ([]ssh.Signer, error) {
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
func (ca *CertAuthorityV1) Check() error {
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

// CertAuthoritySpecV1 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthoritySpecV1 struct {
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
	SigningKeys [][]byte `json:"signing_keys"`
	// Roles is a list of roles assumed by users signed by this CA
	Roles []string `json:"roles"`
}

// CertAuthoritySpecV1Schema is JSON schema for cert authority V1
const CertAuthoritySpecV1Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["type", "cluster_name", "checking_keys", "signing_keys"],
  "properties": {
    "type": {"type": "string"},
    "cluster_name": {"type": "string"},
    "checking_keys": {
      "type": "array",
      "items": {
        "type": "string",
        "format": "base64"
      }
    },
    "signing_keys": {
      "type": "array",
      "items": {
        "type": "string",
        "format": "base64"
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

// CertAuthorityV0 is a host or user certificate authority that
// can check and if it has private key stored as well, sign it too
type CertAuthorityV0 struct {
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

func (c *CertAuthorityV0) V1() *CertAuthorityV1 {
	return &CertAuthorityV1{
		Kind:    KindCertAuthority,
		Version: V1,
		Metadata: Metadata{
			Name: u.Name,
		},
		Spec: CertAuthoritySpecV1{
			Type:         c.Type,
			ClusterName:  c.DomainName,
			Roles:        c.Roles,
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
	UnmarshalCertauthority(bytes []byte) (CertAuthority, error)
	// MarshalCertAuthority to binary representation
	MarshalCertAuthority(c CertAuthority) ([]byte, error)
	// GenerateUser generates new user based on standard teleport user
	// it gives external implementations to add more app-specific
	// data to the user
	GenerateUser(User) (User, error)
}

// GetCertAuthoritySchema returns JSON Schema for cert authorities
func GetCertAuthoritySchema() string {
	return fmt.Sprintf(V1SchemaTemplate, MetadataSchema, CertAuthoritySpecV1Schema)
}

type TeleportCertAuthorityMarshaler struct{}

// UnmarshalUser unmarshals user from JSON
func (*TeleportCertAuthorityMarshaler) UnmarshalCertAuthority(bytes []byte) (User, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var ca CertAuthorityV0
		err := json.Unmarshal(bytes, &ca)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return ca.V1(), nil
	case V1:
		var ca CertAuthorityV1
		if err := utils.UnmarshalWithSchema(GetCertAuthoritySchema(), &ca, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// MarshalUser marshalls cert authority into JSON
func (*TeleportCertAuthorityMarshaler) MarshalCertAuthority(ca CertAuthority) ([]byte, error) {
	return json.Marshal(u)
}
