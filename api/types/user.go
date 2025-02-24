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
	"fmt"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils"
)

// UserType is the user's types that indicates where it was created.
type UserType string

const (
	// UserTypeSSO identifies a user that was created from an SSO provider.
	UserTypeSSO UserType = "sso"
	// UserTypeLocal identifies a user that was created in Teleport itself and has no connection to an external identity.
	UserTypeLocal UserType = "local"
)

// Match checks if the given user matches this filter.
func (f *UserFilter) Match(user *UserV2) bool {
	if len(f.SearchKeywords) != 0 {
		if !user.MatchSearch(f.SearchKeywords) {
			return false
		}
	}

	return true
}

// User represents teleport embedded user or external user.
type User interface {
	// ResourceWithSecrets provides common resource properties
	ResourceWithSecrets
	ResourceWithOrigin
	ResourceWithLabels
	// SetMetadata sets object metadata
	SetMetadata(meta Metadata)
	// GetOIDCIdentities returns a list of connected OIDC identities
	GetOIDCIdentities() []ExternalIdentity
	// GetSAMLIdentities returns a list of connected SAML identities
	GetSAMLIdentities() []ExternalIdentity
	// GetGithubIdentities returns a list of connected Github identities
	GetGithubIdentities() []ExternalIdentity
	// SetGithubIdentities sets the list of connected GitHub identities
	SetGithubIdentities([]ExternalIdentity)
	// Get local authentication secrets (may be nil).
	GetLocalAuth() *LocalAuthSecrets
	// Set local authentication secrets (use nil to delete).
	SetLocalAuth(auth *LocalAuthSecrets)
	// GetRoles returns a list of roles assigned to user
	GetRoles() []string
	// GetLogins gets the list of server logins/principals for the user
	GetLogins() []string
	// GetDatabaseUsers gets the list of Database Users for the user
	GetDatabaseUsers() []string
	// GetDatabaseNames gets the list of Database Names for the user
	GetDatabaseNames() []string
	// GetKubeUsers gets the list of Kubernetes Users for the user
	GetKubeUsers() []string
	// GetKubeGroups gets the list of Kubernetes Groups for the user
	GetKubeGroups() []string
	// GetWindowsLogins gets the list of Windows Logins for the user
	GetWindowsLogins() []string
	// GetAWSRoleARNs gets the list of AWS role ARNs for the user
	GetAWSRoleARNs() []string
	// GetAzureIdentities gets a list of Azure identities for the user
	GetAzureIdentities() []string
	// GetGCPServiceAccounts gets a list of GCP service accounts for the user
	GetGCPServiceAccounts() []string
	// String returns user
	String() string
	// GetStatus return user login status
	GetStatus() LoginStatus
	// SetLocked sets login status to locked
	SetLocked(until time.Time, reason string)
	// ResetLocks resets lock related fields to empty values.
	ResetLocks()
	// SetRoles sets user roles
	SetRoles(roles []string)
	// AddRole adds role to the users' role list
	AddRole(name string)
	// SetLogins sets a list of server logins/principals for user
	SetLogins(logins []string)
	// SetDatabaseUsers sets a list of Database Users for user
	SetDatabaseUsers(databaseUsers []string)
	// SetDatabaseNames sets a list of Database Names for user
	SetDatabaseNames(databaseNames []string)
	// SetDatabaseRoles sets a list of Database roles for user
	SetDatabaseRoles(databaseRoles []string)
	// SetKubeUsers sets a list of Kubernetes Users for user
	SetKubeUsers(kubeUsers []string)
	// SetKubeGroups sets a list of Kubernetes Groups for user
	SetKubeGroups(kubeGroups []string)
	// SetWindowsLogins sets a list of Windows Logins for user
	SetWindowsLogins(logins []string)
	// SetAWSRoleARNs sets a list of AWS role ARNs for user
	SetAWSRoleARNs(awsRoleARNs []string)
	// SetAzureIdentities sets a list of Azure identities for the user
	SetAzureIdentities(azureIdentities []string)
	// SetGCPServiceAccounts sets a list of GCP service accounts for the user
	SetGCPServiceAccounts(accounts []string)
	// SetHostUserUID sets the UID for host users
	SetHostUserUID(uid string)
	// SetHostUserGID sets the GID for host users
	SetHostUserGID(gid string)
	// GetCreatedBy returns information about user
	GetCreatedBy() CreatedBy
	// SetCreatedBy sets created by information
	SetCreatedBy(CreatedBy)
	// GetUserType indicates if the User was created by an SSO Provider or locally.
	GetUserType() UserType
	// GetTraits gets the trait map for this user used to populate role variables.
	GetTraits() map[string][]string
	// SetTraits sets the trait map for this user used to populate role variables.
	SetTraits(map[string][]string)
	// GetTrustedDeviceIDs returns the IDs of the user's trusted devices.
	GetTrustedDeviceIDs() []string
	// SetTrustedDeviceIDs assigns the IDs of the user's trusted devices.
	SetTrustedDeviceIDs(ids []string)
	// IsBot returns true if the user is a bot.
	IsBot() bool
	// BotGenerationLabel returns the bot generation label.
	BotGenerationLabel() string
	// GetPasswordState reflects what the system knows about the user's password.
	// Note that this is a "best effort" property, in that it can be UNSPECIFIED
	// for users who were created before this property was introduced and didn't
	// perform any password-related activity since then. See RFD 0159 for details.
	// Do NOT use this value for authentication purposes!
	GetPasswordState() PasswordState
	// SetPasswordState updates the information about user's password. Note that
	// this is a "best effort" property, in that it can be UNSPECIFIED for users
	// who were created before this property was introduced and didn't perform any
	// password-related activity since then. See RFD 0159 for details.
	SetPasswordState(PasswordState)
	// SetWeakestDevice sets the MFA state for the user.
	SetWeakestDevice(MFADeviceKind)
	// GetWeakestDevice gets the MFA state for the user.
	GetWeakestDevice() MFADeviceKind
}

// NewUser creates new empty user
func NewUser(name string) (User, error) {
	u := &UserV2{
		Metadata: Metadata{
			Name: name,
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

// GetRevision returns the revision
func (u *UserV2) GetRevision() string {
	return u.Metadata.GetRevision()
}

// SetRevision sets the revision
func (u *UserV2) SetRevision(rev string) {
	u.Metadata.SetRevision(rev)
}

// GetMetadata returns object metadata
func (u *UserV2) GetMetadata() Metadata {
	return u.Metadata
}

// Origin returns the origin value of the resource.
func (u *UserV2) Origin() string {
	return u.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (u *UserV2) SetOrigin(origin string) {
	u.Metadata.SetOrigin(origin)
}

// GetLabel fetches the given user label, with the same semantics
// as a map read
func (u *UserV2) GetLabel(key string) (value string, ok bool) {
	value, ok = u.Metadata.Labels[key]
	return
}

// GetAllLabels fetches all the user labels.
func (u *UserV2) GetAllLabels() map[string]string {
	return u.Metadata.Labels
}

// GetStaticLabels fetches all the user labels.
func (u *UserV2) GetStaticLabels() map[string]string {
	return u.Metadata.Labels
}

// SetStaticLabels sets the entire label set for the user.
func (u *UserV2) SetStaticLabels(sl map[string]string) {
	u.Metadata.Labels = sl
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (u *UserV2) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(u.Metadata.Labels), u.GetName())
	return MatchSearch(fieldVals, values, nil)
}

// SetMetadata sets object metadata
func (u *UserV2) SetMetadata(meta Metadata) {
	u.Metadata = meta
}

// SetExpiry sets expiry time for the object
func (u *UserV2) SetExpiry(expires time.Time) {
	u.Metadata.SetExpiry(expires)
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

// GetTrustedDeviceIDs returns the IDs of the user's trusted devices.
func (u *UserV2) GetTrustedDeviceIDs() []string {
	return u.Spec.TrustedDeviceIDs
}

// SetTrustedDeviceIDs assigns the IDs of the user's trusted devices.
func (u *UserV2) SetTrustedDeviceIDs(ids []string) {
	u.Spec.TrustedDeviceIDs = ids
}

// setStaticFields sets static resource header and metadata fields.
func (u *UserV2) setStaticFields() {
	u.Kind = KindUser
	u.Version = V2
}

// CheckAndSetDefaults checks and set default values for any missing fields.
func (u *UserV2) CheckAndSetDefaults() error {
	u.setStaticFields()
	if err := u.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	for _, id := range u.Spec.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
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

func (u *UserV2) setTrait(trait string, list []string) {
	if u.Spec.Traits == nil {
		u.Spec.Traits = make(map[string][]string)
	}
	u.Spec.Traits[trait] = utils.Deduplicate(list)
}

// SetLogins sets the Logins trait for the user
func (u *UserV2) SetLogins(logins []string) {
	u.setTrait(constants.TraitLogins, logins)
}

// SetDatabaseUsers sets the DatabaseUsers trait for the user
func (u *UserV2) SetDatabaseUsers(databaseUsers []string) {
	u.setTrait(constants.TraitDBUsers, databaseUsers)
}

// SetDatabaseNames sets the DatabaseNames trait for the user
func (u *UserV2) SetDatabaseNames(databaseNames []string) {
	u.setTrait(constants.TraitDBNames, databaseNames)
}

// SetDatabaseRoles sets the DatabaseRoles trait for the user
func (u *UserV2) SetDatabaseRoles(databaseRoles []string) {
	u.setTrait(constants.TraitDBRoles, databaseRoles)
}

// SetKubeUsers sets the KubeUsers trait for the user
func (u *UserV2) SetKubeUsers(kubeUsers []string) {
	u.setTrait(constants.TraitKubeUsers, kubeUsers)
}

// SetKubeGroups sets the KubeGroups trait for the user
func (u *UserV2) SetKubeGroups(kubeGroups []string) {
	u.setTrait(constants.TraitKubeGroups, kubeGroups)
}

// SetWindowsLogins sets the WindowsLogins trait for the user
func (u *UserV2) SetWindowsLogins(logins []string) {
	u.setTrait(constants.TraitWindowsLogins, logins)
}

// SetAWSRoleARNs sets the AWSRoleARNs trait for the user
func (u *UserV2) SetAWSRoleARNs(awsRoleARNs []string) {
	u.setTrait(constants.TraitAWSRoleARNs, awsRoleARNs)
}

// SetAzureIdentities sets a list of Azure identities for the user
func (u *UserV2) SetAzureIdentities(identities []string) {
	u.setTrait(constants.TraitAzureIdentities, identities)
}

// SetGCPServiceAccounts sets a list of GCP service accounts for the user
func (u *UserV2) SetGCPServiceAccounts(accounts []string) {
	u.setTrait(constants.TraitGCPServiceAccounts, accounts)
}

// SetHostUserUID sets the host user UID
func (u *UserV2) SetHostUserUID(uid string) {
	u.setTrait(constants.TraitHostUserUID, []string{uid})
}

// SetHostUserGID sets the host user GID
func (u *UserV2) SetHostUserGID(uid string) {
	u.setTrait(constants.TraitHostUserGID, []string{uid})
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

// SetGithubIdentities sets the list of connected GitHub identities
func (u *UserV2) SetGithubIdentities(identities []ExternalIdentity) {
	u.Spec.GithubIdentities = identities
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

func (u UserV2) getTrait(trait string) []string {
	if u.Spec.Traits == nil {
		return []string{}
	}
	return u.Spec.Traits[trait]
}

// GetLogins gets the list of server logins/principals for the user
func (u UserV2) GetLogins() []string {
	return u.getTrait(constants.TraitLogins)
}

// GetDatabaseUsers gets the list of DB Users for the user
func (u UserV2) GetDatabaseUsers() []string {
	return u.getTrait(constants.TraitDBUsers)
}

// GetDatabaseNames gets the list of DB Names for the user
func (u UserV2) GetDatabaseNames() []string {
	return u.getTrait(constants.TraitDBNames)
}

// GetKubeUsers gets the list of Kubernetes Users for the user
func (u UserV2) GetKubeUsers() []string {
	return u.getTrait(constants.TraitKubeUsers)
}

// GetKubeGroups gets the list of Kubernetes Groups for the user
func (u UserV2) GetKubeGroups() []string {
	return u.getTrait(constants.TraitKubeGroups)
}

// GetWindowsLogins gets the list of Windows Logins for the user
func (u UserV2) GetWindowsLogins() []string {
	return u.getTrait(constants.TraitWindowsLogins)
}

// GetAWSRoleARNs gets the list of AWS role ARNs for the user
func (u UserV2) GetAWSRoleARNs() []string {
	return u.getTrait(constants.TraitAWSRoleARNs)
}

// GetAzureIdentities gets a list of Azure identities for the user
func (u UserV2) GetAzureIdentities() []string {
	return u.getTrait(constants.TraitAzureIdentities)
}

// GetGCPServiceAccounts gets a list of GCP service accounts for the user
func (u UserV2) GetGCPServiceAccounts() []string {
	return u.getTrait(constants.TraitGCPServiceAccounts)
}

// GetUserType indicates if the User was created by an SSO Provider or locally.
func (u UserV2) GetUserType() UserType {
	if u.GetCreatedBy().Connector != nil ||
		len(u.GetOIDCIdentities()) > 0 ||
		len(u.GetGithubIdentities()) > 0 ||
		len(u.GetSAMLIdentities()) > 0 {

		return UserTypeSSO
	}

	return UserTypeLocal
}

// IsBot returns true if the user is a bot.
func (u UserV2) IsBot() bool {
	_, ok := u.GetMetadata().Labels[BotLabel]
	return ok
}

// BotGenerationLabel returns the bot generation label.
func (u UserV2) BotGenerationLabel() string {
	return u.GetMetadata().Labels[BotGenerationLabel]
}

func (u *UserV2) String() string {
	return fmt.Sprintf("User(name=%v, roles=%v, identities=%v)", u.Metadata.Name, u.Spec.Roles, u.Spec.OIDCIdentities)
}

// SetLocked marks the user as locked
func (u *UserV2) SetLocked(until time.Time, reason string) {
	u.Spec.Status.IsLocked = true
	u.Spec.Status.LockExpires = until
	u.Spec.Status.LockedMessage = reason
	u.Spec.Status.LockedTime = time.Now().UTC()
}

// ResetLocks resets lock related fields to empty values.
func (u *UserV2) ResetLocks() {
	u.Spec.Status.IsLocked = false
	u.Spec.Status.LockedMessage = ""
	u.Spec.Status.LockExpires = time.Time{}
}

// DeepCopy creates a clone of this user value.
func (u *UserV2) DeepCopy() User {
	return utils.CloneProtoMsg(u)
}

func (u *UserV2) GetPasswordState() PasswordState {
	return u.Status.PasswordState
}

func (u *UserV2) SetPasswordState(state PasswordState) {
	u.Status.PasswordState = state
}

func (u *UserV2) SetWeakestDevice(state MFADeviceKind) {
	u.Status.MfaWeakestDevice = state
}

func (u *UserV2) GetWeakestDevice() MFADeviceKind {
	return u.Status.MfaWeakestDevice
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
