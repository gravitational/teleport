/*
Copyright 2020 Gravitational, Inc.

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

package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// User represents teleport embedded user or external user
type User interface {
	// ResourceWithSecrets provides common resource properties
	ResourceWithSecrets
	// GetOIDCIdentities returns a list of connected OIDC identities
	GetOIDCIdentities() []ExternalIdentity
	// GetSAMLIdentities returns a list of connected SAML identities
	GetSAMLIdentities() []ExternalIdentity
	// GetGithubIdentities returns a list of connected Github identities
	GetGithubIdentities() []ExternalIdentity
	// Get local authentication secrets (may be nil).
	GetLocalAuth() *LocalAuthSecrets
	// Set local authentication secrets (use nil to delete).
	SetLocalAuth(auth *LocalAuthSecrets)
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
	// GetCreatedBy returns information about user
	GetCreatedBy() CreatedBy
	// SetCreatedBy sets created by information
	SetCreatedBy(CreatedBy)
	// Check checks basic user parameters for errors
	Check() error
	// GetTraits gets the trait map for this user used to populate role variables.
	GetTraits() map[string][]string
	// GetTraits sets the trait map for this user used to populate role variables.
	SetTraits(map[string][]string)
	// CheckAndSetDefaults checks and set default values for any missing fields.
	CheckAndSetDefaults() error
}

// NewUser creates new empty user
func NewUser(name string) (User, error) {
	u := &UserV2{
		Kind:    KindUser,
		Version: V2,
		Metadata: Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
	}
	if err := u.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// IsSameProvider returns true if the provided connector has the
// same ID/type as this one
func (r *ConnectorRef) IsSameProvider(other *ConnectorRef) bool {
	return other != nil && other.Type == r.Type && other.ID == r.ID
}

// GetVersion returns resource version
func (u *UserV2) GetVersion() string {
	return u.Version
}

// GetKind returns resource kind
func (u *UserV2) GetKind() string {
	return u.Kind
}

// GetSubKind returns resource sub kind
func (u *UserV2) GetSubKind() string {
	return u.SubKind
}

// SetSubKind sets resource subkind
func (u *UserV2) SetSubKind(s string) {
	u.SubKind = s
}

// GetResourceID returns resource ID
func (u *UserV2) GetResourceID() int64 {
	return u.Metadata.ID
}

// SetResourceID sets resource ID
func (u *UserV2) SetResourceID(id int64) {
	u.Metadata.ID = id
}

// GetMetadata returns object metadata
func (u *UserV2) GetMetadata() Metadata {
	return u.Metadata
}

// SetExpiry sets expiry time for the object
func (u *UserV2) SetExpiry(expires time.Time) {
	u.Metadata.SetExpiry(expires)
}

// SetTTL sets Expires header using realtime clock
func (u *UserV2) SetTTL(clock Clock, ttl time.Duration) {
	u.Metadata.SetTTL(clock, ttl)
}

// GetName returns the name of the User
func (u *UserV2) GetName() string {
	return u.Metadata.Name
}

// SetName sets the name of the User
func (u *UserV2) SetName(e string) {
	u.Metadata.Name = e
}

// WithoutSecrets returns an instance of resource without secrets.
func (u *UserV2) WithoutSecrets() Resource {
	if u.Spec.LocalAuth == nil {
		return u
	}
	u2 := *u
	u2.Spec.LocalAuth = nil
	return &u2
}

// GetTraits gets the trait map for this user used to populate role variables.
func (u *UserV2) GetTraits() map[string][]string {
	return u.Spec.Traits
}

// SetTraits sets the trait map for this user used to populate role variables.
func (u *UserV2) SetTraits(traits map[string][]string) {
	u.Spec.Traits = traits
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u *UserV2) CheckAndSetDefaults() error {
	err := u.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = u.Check()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// SetCreatedBy sets created by information
func (u *UserV2) SetCreatedBy(b CreatedBy) {
	u.Spec.CreatedBy = b
}

// GetCreatedBy returns information about who created user
func (u *UserV2) GetCreatedBy() CreatedBy {
	return u.Spec.CreatedBy
}

// Equals checks if user equals to another
func (u *UserV2) Equals(other User) bool {
	if u.Metadata.Name != other.GetName() {
		return false
	}
	otherIdentities := other.GetOIDCIdentities()
	if len(u.Spec.OIDCIdentities) != len(otherIdentities) {
		return false
	}
	for i := range u.Spec.OIDCIdentities {
		if !u.Spec.OIDCIdentities[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	otherSAMLIdentities := other.GetSAMLIdentities()
	if len(u.Spec.SAMLIdentities) != len(otherSAMLIdentities) {
		return false
	}
	for i := range u.Spec.SAMLIdentities {
		if !u.Spec.SAMLIdentities[i].Equals(&otherSAMLIdentities[i]) {
			return false
		}
	}
	otherGithubIdentities := other.GetGithubIdentities()
	if len(u.Spec.GithubIdentities) != len(otherGithubIdentities) {
		return false
	}
	for i := range u.Spec.GithubIdentities {
		if !u.Spec.GithubIdentities[i].Equals(&otherGithubIdentities[i]) {
			return false
		}
	}
	return u.Spec.LocalAuth.Equals(other.GetLocalAuth())
}

// Expiry returns expiry time for temporary users. Prefer expires from
// metadata, if it does not exist, fall back to expires in spec.
func (u *UserV2) Expiry() time.Time {
	if u.Metadata.Expires != nil && !u.Metadata.Expires.IsZero() {
		return *u.Metadata.Expires
	}
	return u.Spec.Expires
}

// SetRoles sets a list of roles for user
func (u *UserV2) SetRoles(roles []string) {
	u.Spec.Roles = utils.Deduplicate(roles)
}

// GetStatus returns login status of the user
func (u *UserV2) GetStatus() LoginStatus {
	return u.Spec.Status
}

// GetOIDCIdentities returns a list of connected OIDC identities
func (u *UserV2) GetOIDCIdentities() []ExternalIdentity {
	return u.Spec.OIDCIdentities
}

// GetSAMLIdentities returns a list of connected SAML identities
func (u *UserV2) GetSAMLIdentities() []ExternalIdentity {
	return u.Spec.SAMLIdentities
}

// GetGithubIdentities returns a list of connected Github identities
func (u *UserV2) GetGithubIdentities() []ExternalIdentity {
	return u.Spec.GithubIdentities
}

// GetLocalAuth gets local authentication secrets (may be nil).
func (u *UserV2) GetLocalAuth() *LocalAuthSecrets {
	return u.Spec.LocalAuth
}

// SetLocalAuth sets local authentication secrets (use nil to delete).
func (u *UserV2) SetLocalAuth(auth *LocalAuthSecrets) {
	u.Spec.LocalAuth = auth
}

// GetRoles returns a list of roles assigned to user
func (u *UserV2) GetRoles() []string {
	return u.Spec.Roles
}

// AddRole adds a role to user's role list
func (u *UserV2) AddRole(name string) {
	for _, r := range u.Spec.Roles {
		if r == name {
			return
		}
	}
	u.Spec.Roles = append(u.Spec.Roles, name)
}

func (u *UserV2) String() string {
	return fmt.Sprintf("User(name=%v, roles=%v, identities=%v)", u.Metadata.Name, u.Spec.Roles, u.Spec.OIDCIdentities)
}

// SetLocked marks the user as locked
func (u *UserV2) SetLocked(until time.Time, reason string) {
	u.Spec.Status.IsLocked = true
	u.Spec.Status.LockExpires = until
	u.Spec.Status.LockedMessage = reason
}

// Check checks validity of all parameters
func (u *UserV2) Check() error {
	if u.Kind == "" {
		return trace.BadParameter("user kind is not set")
	}
	if u.Version == "" {
		return trace.BadParameter("user version is not set")
	}
	if u.Metadata.Name == "" {
		return trace.BadParameter("user name cannot be empty")
	}
	for _, id := range u.Spec.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	if localAuth := u.GetLocalAuth(); localAuth != nil {
		if err := localAuth.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

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

// String returns debug friendly representation of this identity
func (i *ExternalIdentity) String() string {
	return fmt.Sprintf("OIDCIdentity(connectorID=%v, username=%v)", i.ConnectorID, i.Username)
}

// Equals returns true if this identity equals to passed one
func (i *ExternalIdentity) Equals(other *ExternalIdentity) bool {
	return i.ConnectorID == other.ConnectorID && i.Username == other.Username
}

// Check returns nil if all parameters are great, err otherwise
func (i *ExternalIdentity) Check() error {
	if i.ConnectorID == "" {
		return trace.BadParameter("ConnectorID: missing value")
	}
	if i.Username == "" {
		return trace.BadParameter("Username: missing username")
	}
	return nil
}

// UserSpecV2SchemaTemplate is JSON schema for V2 user
const UserSpecV2SchemaTemplate = `{
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
		"traits": {
			"type": "object",
			"additionalProperties": false,
			"patternProperties": {
				"^.+$": {
					"type": ["array", "null"],
					"items": {
						"type": "string"
					}
				}
			}
		},
		"oidc_identities": {
			"type": "array",
			"items": %v
		},
		"saml_identities": {
			"type": "array",
			"items": %v
		},
		"github_identities": {
			"type": "array",
			"items": %v
		},
		"status": %v,
		"created_by": %v,
		"local_auth": %v%v
	}
}`

// CreatedBySchema is JSON schema for CreatedBy
const CreatedBySchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"connector": {
			"additionalProperties": false,
			"type": "object",
			"properties": {
			"type": {"type": "string"},
			"id": {"type": "string"},
			"identity": {"type": "string"}
			}
		},
		"time": {"type": "string"},
		"user": {
			"type": "object",
			"additionalProperties": false,
			"properties": {"name": {"type": "string"}}
		}
	}
}`

// ExternalIdentitySchema is JSON schema for ExternalIdentity
const ExternalIdentitySchema = `{
	"type": "object",
	"additionalProperties": false,
	"properties": {
		"connector_id": {"type": "string"},
		"username": {"type": "string"}
	}
}`

// LoginStatusSchema is JSON schema for LoginStatus
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

// GetUserSchema returns role schema with optionally injected
// schema for extensions
func GetUserSchema(extensionSchema string) string {
	var userSchema string
	if extensionSchema == "" {
		userSchema = fmt.Sprintf(UserSpecV2SchemaTemplate, ExternalIdentitySchema, ExternalIdentitySchema, ExternalIdentitySchema, LoginStatusSchema, CreatedBySchema, LocalAuthSecretsSchema, ``)
	} else {
		userSchema = fmt.Sprintf(UserSpecV2SchemaTemplate, ExternalIdentitySchema, ExternalIdentitySchema, ExternalIdentitySchema, LoginStatusSchema, CreatedBySchema, LocalAuthSecretsSchema, ", "+extensionSchema)
	}
	return fmt.Sprintf(V2SchemaTemplate, MetadataSchema, userSchema, DefaultDefinitions)
}

// UserMarshaler implements marshal/unmarshal of User implementations
// mostly adds support for extended versions
type UserMarshaler interface {
	// UnmarshalUser from binary representation
	UnmarshalUser(bytes []byte, opts ...MarshalOption) (User, error)
	// MarshalUser to binary representation
	MarshalUser(u User, opts ...MarshalOption) ([]byte, error)
	// GenerateUser generates new user based on standard teleport user
	// it gives external implementations to add more app-specific
	// data to the user
	GenerateUser(User) (User, error)
}

type teleportUserMarshaler struct{}

// UnmarshalUser unmarshals user from JSON
func (*teleportUserMarshaler) UnmarshalUser(bytes []byte, opts ...MarshalOption) (User, error) {
	var h ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch h.Version {
	case V2:
		var u UserV2
		if cfg.SkipValidation {
			if err := utils.FastUnmarshal(bytes, &u); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		} else {
			if err := utils.UnmarshalWithSchema(GetUserSchema(""), &u, bytes); err != nil {
				return nil, trace.BadParameter(err.Error())
			}
		}

		if err := u.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.ID != 0 {
			u.SetResourceID(cfg.ID)
		}
		if !cfg.Expires.IsZero() {
			u.SetExpiry(cfg.Expires)
		}

		return &u, nil
	}
	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// MarshalUser marshalls user into JSON
func (*teleportUserMarshaler) MarshalUser(u User, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch user := u.(type) {
	case *UserV2:
		if !cfg.PreserveResourceID {
			// avoid modifying the original object
			// to prevent unexpected data races
			copy := *user
			copy.SetResourceID(0)
			user = &copy
		}
		return utils.FastMarshal(user)
	default:
		return nil, trace.BadParameter("unrecognized user version %T", u)
	}
}

// GenerateUser generates new user
func (*teleportUserMarshaler) GenerateUser(in User) (User, error) {
	return in, nil
}

var userMarshaler UserMarshaler = &teleportUserMarshaler{}

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
