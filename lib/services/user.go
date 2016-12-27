package services

import (
	"time"
)

var mtx sync.Mutex
var userMarshaler UserMarshaler = &TeleportUserMarshaler{}

// SetUserMarshaler sets global user marshaler
func SetUserMarshaler(u UserMarshaler) {
	mtx.Lock()
	defer mtx.Unlock()
	userMarshaler = u
}

// GetUserMarshaler returns currently set user marshaler
func GetUserMarshaler() UserMarshaler {
	mtx.Lock()
	defer mtx.Unlock()
	return userMarshaler
}

// UserMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type UserMarshaler interface {
	// UnmarshalUser from binary representation
	UnmarshalUser(bytes []byte) (User, error)
	// MarshalUser to binary representation
	MarshalUser(u User) ([]byte, error)
	// GenerateUser generates new user based on standard teleport user
	// it gives external implementations to add more app-specific
	// data to the user
	GenerateUser(User) (User, error)
}

type TeleportUserMarshaler struct{}

// UnmarshalUser unmarshals user from JSON
func (*TeleportUserMarshaler) UnmarshalUser(bytes []byte) (User, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var u UserV0
		err := json.Unmarshal(bytes, &u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return u.V1(), nil
	}

	return u, nil
}

// GenerateUser generates new user
func (*TeleportUserMarshaler) GenerateUser(in TeleportUser) (User, error) {
	return &in, nil
}

// MarshalUser marshalls user into JSON
func (*TeleportUserMarshaler) MarshalUser(u User) ([]byte, error) {
	return json.Marshal(u)
}

// UserV1 is version1 resource spec of the user
type UserV1 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is User metadata
	Metadata Metadata `json:"metadata"`
	// Spec contains user specification
	Spec UserV1Spec `json:"spec"`
}

// UserV1SchemaTemplate is a template JSON Schema for user
const UserV1SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "required": ["kind", "spec", "metadata"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
    "metadata": %v,
    "spec": %v
  }
}`

// UserSpecV1 is a specification for V1 user
type UserSpecV1 struct {
	// AllowedLogins represents a list of OS users this teleport
	// user is allowed to login as
	AllowedLogins []string `json:"allowed_logins"`

	// OIDCIdentities lists associated OpenID Connect identities
	// that let user log in using externally verified identity
	OIDCIdentities []OIDCIdentity `json:"oidc_identities"`

	// Roles is a list of roles assigned to user
	Roles []string `json:"roles"`

	// Status is a login status of the user
	Status LoginStatus `json:"status"`

	// Expires if set sets TTL on the user
	Expires time.Time `json:"expires"`

	// CreatedBy holds information about agent or person created this usre
	CreatedBy CreatedBy `json:"created_by"`
}

// UserSpecV1SchemaTemplate is JSON schema for
var UserSpecV1Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "allowed_logins": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "oidc_identities": {
      "type": "array",
      "items": %v
    },
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "status": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
         
      }
    }
  }
}`, OIDCIdentitySchema, LoginStatusSchema, CreatedBySchema)

// SetCreatedBy sets created by information
func (u *UserV1) SetCreatedBy(b CreatedBy) {
	u.CreatedBy = b
}

// GetCreatedBy returns information about who created user
func (u *UserV1) GetCreatedBy() CreatedBy {
	return u.CreatedBy
}

// Equals checks if user equals to another
func (u *UserV1) Equals(other User) bool {
	if u.Name != other.GetName() {
		return false
	}
	otherLogins := other.GetAllowedLogins()
	if len(u.AllowedLogins) != len(otherLogins) {
		return false
	}
	for i := range u.AllowedLogins {
		if u.AllowedLogins[i] != otherLogins[i] {
			return false
		}
	}
	otherIdentities := other.GetIdentities()
	if len(u.OIDCIdentities) != len(otherIdentities) {
		return false
	}
	for i := range u.OIDCIdentities {
		if !u.OIDCIdentities[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	return true
}

// GetExpiry returns expiry time for temporary users
func (u *UserV1) GetExpiry() time.Time {
	return u.Expires
}

// SetAllowedLogins sets allowed logins for user
func (u *UserV1) SetAllowedLogins(logins []string) {
	u.AllowedLogins = logins
}

// SetRoles sets a list of roles for user
func (u *UserV1) SetRoles(roles []string) {
	u.Roles = utils.Deduplicate(roles)
}

// GetStatus returns login status of the user
func (u *UserV1) GetStatus() LoginStatus {
	return u.Status
}

// WebSessionInfo returns web session information
func (u *UserV1) WebSessionInfo(logins []string) User {
	c := *u
	c.AllowedLogins = logins
	return &c
}

// GetAllowedLogins returns user's allowed linux logins
func (u *UserV1) GetAllowedLogins() []string {
	return u.AllowedLogins
}

// GetIdentities returns a list of connected OIDCIdentities
func (u *UserV1) GetIdentities() []OIDCIdentity {
	return u.OIDCIdentities
}

// GetRoles returns a list of roles assigned to user
func (u *UserV1) GetRoles() []string {
	return u.Roles
}

// AddRole adds a role to user's role list
func (u *UserV1) AddRole(name string) {
	for _, r := range u.Roles {
		if r == name {
			return
		}
	}
	u.Roles = append(u.Roles, name)
}

// GetName returns user name
func (u *UserV1) GetName() string {
	return u.Name
}

func (u *UserV1) String() string {
	return fmt.Sprintf("User(name=%v, allowed_logins=%v, identities=%v)", u.Name, u.AllowedLogins, u.OIDCIdentities)
}

func (u *UserV1) SetLocked(until time.Time, reason string) {
	u.Status.IsLocked = true
	u.Status.LockExpires = until
	u.Status.LockedMessage = reason
}

// Check checks validity of all parameters
func (u *UserV1) Check() error {
	if !cstrings.IsValidUnixUser(u.Name) {
		return trace.BadParameter("'%v' is not a valid user name", u.Name)
	}
	for _, l := range u.AllowedLogins {
		if !cstrings.IsValidUnixUser(l) {
			return trace.BadParameter("'%v is not a valid unix username'", l)
		}
	}
	for _, login := range u.AllowedLogins {
		if !cstrings.IsValidUnixUser(login) {
			return trace.BadParameter("'%v' is not a valid user name", login)
		}
	}
	for _, id := range u.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UserV0 is V0 version of the user
type UserV0 struct {
	// Name is a user name
	Name string `json:"name"`

	// AllowedLogins represents a list of OS users this teleport
	// user is allowed to login as
	AllowedLogins []string `json:"allowed_logins"`

	// OIDCIdentities lists associated OpenID Connect identities
	// that let user log in using externally verified identity
	OIDCIdentities []OIDCIdentity `json:"oidc_identities"`

	// Roles is a list of roles assigned to user
	Roles []string `json:"roles"`

	// Status is a login status of the user
	Status LoginStatus `json:"status"`

	// Expires if set sets TTL on the user
	Expires time.Time `json:"expires"`

	// CreatedBy holds information about agent or person created this usre
	CreatedBy CreatedBy `json:"created_by"`
}

// UserV1 converts UserV0 to UserV1 format
func (u *UserV0) UserV1() *UserV1 {
	return &UserV1{
		Kind:    KindUser,
		Version: V1,
		Metadata: Metadata{
			Name: u.Name,
		},
		Spec: UserSpecV1{
			AllowedLogins:  u.AllowedLogins,
			OIDCIDentities: u.OIDCIdentitties,
			Roles:          u.Roles,
			Status:         u.Status,
			Expires:        u.Expires,
			CreatedBy:      u.CreatedBy,
		},
	}
}
