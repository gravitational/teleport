package services

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

// User represents teleport embedded user or external user
type User interface {
	// GetName returns user name
	GetName() string
	// GetIdentities returns a list of connected OIDCIdentities
	GetIdentities() []OIDCIdentity
	// GetRoles returns a list of roles assigned to user
	GetRoles() []string
	// String returns user
	String() string
	// Equals checks if user equals to another
	Equals(other User) bool
	// GetStatus return user login status
	GetStatus() LoginStatus
	// SetLocked sets login status to locked
	SetLocked(until time.Time, reason string)
	// SetRoles sets user roles
	SetRoles(roles []string)
	// AddRole adds role to the users' role list
	AddRole(name string)
	// GetExpiry returns ttl of the user
	GetExpiry() time.Time
	// GetCreatedBy returns information about user
	GetCreatedBy() CreatedBy
	// SetCreatedBy sets created by information
	SetCreatedBy(CreatedBy)
	// Check checks basic user parameters for errors
	Check() error
	// GetRawObject returns raw object data, used for migrations
	GetRawObject() interface{}
}

// ConnectorRef holds information about OIDC connector
type ConnectorRef struct {
	// Type is connector type
	Type string `json:"connector_type"`
	// ID is connector ID
	ID string `json:"id"`
	// Identity is external identity of the user
	Identity string `json:"identity"`
}

// UserRef holds refernce to user
type UserRef struct {
	// Name is name of the user
	Name string `json:"name"`
}

// CreatedBy holds information about the person or agent who created the user
type CreatedBy struct {
	// Identity if present means that user was automatically created by identity
	Connector *ConnectorRef `json:"connector,omitemtpy"`
	// Time specifies when user was created
	Time time.Time `json:"time"`
	// User holds information about user
	User UserRef `json:"user"`
}

const CreatedBySchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
     "connector": {
       "additionalProperties": false,
       "type": "object"
      },
     "locked_message": {"type": "string"},
     "locked_time": {"type": "string"},
     "lock_expires": {"type": "string"}
   }
}`

// IsEmpty returns true if there's no info about who created this user
func (c CreatedBy) IsEmpty() bool {
	return c.User.Name == ""
}

// String returns human readable information about the user
func (c CreatedBy) String() string {
	if c.User.Name == "" {
		return "system"
	}
	if c.Connector != nil {
		return fmt.Sprintf("%v connector %v for user %v at %v",
			c.Connector.Type, c.Connector.ID, c.Connector.Identity, utils.HumanTimeFormat(c.Time))
	}
	return fmt.Sprintf("%v at %v", c.User.Name, c.Time)
}

// LoginStatus is a login status of the user
type LoginStatus struct {
	// IsLocked tells us if user is locked
	IsLocked bool `json:"is_locked"`
	// LockedMessage contains the message in case if user is locked
	LockedMessage string `json:"locked_message"`
	// LockedTime contains time when user was locked
	LockedTime time.Time `json:"locked_time"`
	// LockExpires contains time when this lock will expire
	LockExpires time.Time `json:"lock_expires"`
}

const LoginStatusSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
     "is_locked": {"type": "boolean"}, 
     "locked_message": {"type": "string"},
     "locked_time": {"type": "string"},
     "lock_expires": {"type": "string"}
   }
}`

// LoginAttempt represents successfull or unsuccessful attempt for user to login
type LoginAttempt struct {
	// Time is time of the attempt
	Time time.Time `json:"time"`
	// Sucess indicates whether attempt was successfull
	Success bool `json:"bool"`
}

// Check checks parameters
func (la *LoginAttempt) Check() error {
	if la.Time.IsZero() {
		return trace.BadParameter("missing parameter time")
	}
	return nil
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
	Spec UserSpecV1 `json:"spec"`
	// rawObject contains raw object representation
	rawObject interface{}
}

// UserSpecV1 is a specification for V1 user
type UserSpecV1 struct {
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
const UserSpecV1SchemaTemplate = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "expires": {"type": "string"},
    "roles": {
      "type": "array",
      "items": {
        "type": "string"
      }
    },
    "oidc_identities": {
      "type": "array",
      "items": %v
    },
    "status": %v,
    "created_by": %v%v
  }
}`

// GetObject returns raw object data, used for migrations
func (u *UserV1) GetRawObject() interface{} {
	return u.rawObject
}

// SetCreatedBy sets created by information
func (u *UserV1) SetCreatedBy(b CreatedBy) {
	u.Spec.CreatedBy = b
}

// GetCreatedBy returns information about who created user
func (u *UserV1) GetCreatedBy() CreatedBy {
	return u.Spec.CreatedBy
}

// Equals checks if user equals to another
func (u *UserV1) Equals(other User) bool {
	if u.Metadata.Name != other.GetName() {
		return false
	}
	otherIdentities := other.GetIdentities()
	if len(u.Spec.OIDCIdentities) != len(otherIdentities) {
		return false
	}
	for i := range u.Spec.OIDCIdentities {
		if !u.Spec.OIDCIdentities[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	return true
}

// GetExpiry returns expiry time for temporary users
func (u *UserV1) GetExpiry() time.Time {
	return u.Spec.Expires
}

// SetRoles sets a list of roles for user
func (u *UserV1) SetRoles(roles []string) {
	u.Spec.Roles = utils.Deduplicate(roles)
}

// GetStatus returns login status of the user
func (u *UserV1) GetStatus() LoginStatus {
	return u.Spec.Status
}

// GetIdentities returns a list of connected OIDCIdentities
func (u *UserV1) GetIdentities() []OIDCIdentity {
	return u.Spec.OIDCIdentities
}

// GetRoles returns a list of roles assigned to user
func (u *UserV1) GetRoles() []string {
	return u.Spec.Roles
}

// AddRole adds a role to user's role list
func (u *UserV1) AddRole(name string) {
	for _, r := range u.Spec.Roles {
		if r == name {
			return
		}
	}
	u.Spec.Roles = append(u.Spec.Roles, name)
}

// GetName returns user name
func (u *UserV1) GetName() string {
	return u.Metadata.Name
}

func (u *UserV1) String() string {
	return fmt.Sprintf("User(name=%v, roles=%v, identities=%v)", u.Metadata.Name, u.Spec.Roles, u.Spec.OIDCIdentities)
}

func (u *UserV1) SetLocked(until time.Time, reason string) {
	u.Spec.Status.IsLocked = true
	u.Spec.Status.LockExpires = until
	u.Spec.Status.LockedMessage = reason
}

// Check checks validity of all parameters
func (u *UserV1) Check() error {
	if !cstrings.IsValidUnixUser(u.Metadata.Name) {
		return trace.BadParameter("'%v' is not a valid user name", u.Metadata.Name)
	}
	for _, id := range u.Spec.OIDCIdentities {
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

//V1 converts UserV0 to UserV1 format
func (u *UserV0) V1() *UserV1 {
	return &UserV1{
		Kind:    KindUser,
		Version: V1,
		Metadata: Metadata{
			Name: u.Name,
		},
		Spec: UserSpecV1{
			OIDCIdentities: u.OIDCIdentities,
			Roles:          u.Roles,
			Status:         u.Status,
			Expires:        u.Expires,
			CreatedBy:      u.CreatedBy,
		},
		rawObject: *u,
	}
}

var userMarshaler UserMarshaler = &TeleportUserMarshaler{}

// SetUserMarshaler sets global user marshaler
func SetUserMarshaler(u UserMarshaler) {
	marshalerMutex.Lock()
	defer marshalerMutex.Unlock()
	userMarshaler = u
}

// GetUserMarshaler returns currently set user marshaler
func GetUserMarshaler() UserMarshaler {
	marshalerMutex.RLock()
	defer marshalerMutex.RUnlock()
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

// GetRoleSchema returns role schema with optionally injected
// schema for extensions
func GetUserSchema(extensionSchema string) string {
	var userSchema string
	if extensionSchema == "" {
		userSchema = fmt.Sprintf(UserSpecV1SchemaTemplate, OIDCIDentitySchema, LoginStatusSchema, CreatedBySchema, `, {"type": "object"}`)
	} else {
		userSchema = fmt.Sprintf(UserSpecV1SchemaTemplate, OIDCIDentitySchema, LoginStatusSchema, CreatedBySchema, ", "+extensionSchema)
	}
	return fmt.Sprintf(V1SchemaTemplate, MetadataSchema, userSchema)
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
	case V1:
		var u UserV1
		if err := utils.UnmarshalWithSchema(GetUserSchema(""), &u, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
	}

	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// GenerateUser generates new user
func (*TeleportUserMarshaler) GenerateUser(in User) (User, error) {
	return in, nil
}

// MarshalUser marshalls user into JSON
func (*TeleportUserMarshaler) MarshalUser(u User) ([]byte, error) {
	return json.Marshal(u)
}
