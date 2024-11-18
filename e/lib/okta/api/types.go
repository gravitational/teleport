package api

import "fmt"

// UserName holds a human-friendly username that should be common between
// Teleport and Okta.
type UserName string

// OktaAppID holds an Okta-generated ID string for an Okta application
type OktaAppID string

// OktaUserID holds an Okta-generated ID string for an Okta user. This is the
// persistent, unique identifier for a given Okta user, as the userName may
// change.
type OktaUserID string

// OktaGroupID holds the Okta-generated ID string for an Okta group. This is the
// persistent ID of a group, as the name and description can be changed.
type OktaGroupID string

// AppAssignmentScope is the assignment scope of the application.
// components/schemas/AppUserProfile Okta API users assignment scope
// A description of this field can be found in the Okta API docs:
// https://developer.okta.com/docs/reference/api/apps/#application-user-object
type AppAssignmentScope string

const (
	// UserScope is the user scope for an application assignment.
	UserScope AppAssignmentScope = "USER"
	// GroupScope is the group scope for an application assignment.
	GroupScope AppAssignmentScope = "GROUP"
)

// UserGroup represents a user group in Okta.
type UserGroup struct {
	// ID is the unique identifier of the group.
	ID string
	// Name is the human-readable name of the group.
	// This is displayed in the Okta UI and is not guaranteed to be unique.
	Name string
}

const (
	// Okta error constants are not housed within the SDK, so we'll need to refer to the
	// documentation directly and define our own.
	// https://developer.okta.com/docs/reference/error-codes/

	OktaErrCodeAPIValidationException        = "E0000001"
	OktaErrCodeAuthenticationException       = "E0000004"
	OktaErrCodeInvalidSessionException       = "E0000005"
	OktaErrCodeAccessDeniedException         = "E0000006"
	OktaErrCodeResourceNotFoundException     = "E0000007"
	OktaErrCodeNotFoundException             = "E0000008"
	OktaErrCodeInvalidTokenProvidedException = "E0000011"
)

// OktaAPIValidationError is a validation error.
type OktaAPIValidationError struct {
	// ErrorID is the unique identifier of the error.
	ErrorID string
	// Summary is a human-readable summary of the error.
	Summary string
}

// Error returns a string representation of the error.
func (o OktaAPIValidationError) Error() string {
	return fmt.Sprintf("%s: %s", o.ErrorID, o.Summary)
}

type EmbeddedLinks struct {
	// AppLinks is a list of links to Okta applications.
	AppLinks []AppLink `mapstructure:"appLinks"`
	// Metadata is a link to the metadata of the application.
	Metadata *AppLink `mapstructure:"metadata"`
}

// AppLink represents a link to an Okta application.
type AppLink struct {
	// Name is the human-readable name of the application.
	Name string `mapstructure:"name"`
	// Href is the URL to the application.
	Href string `mapstructure:"href"`
	Type string `mapstructure:"type"`
}

const (
	// OktaActive indicates an 'active' app or user status, implying that it
	// should be included in application listings.
	OktaActive = "ACTIVE"
	// OktaAdminConsole is the nme of the Okta Admin Console application.
	OktaAdminConsole = "Okta Admin Console"
	// OktaGroupEveryone identifies the default group containing every user
	// in an Okta system. It's always present.
	OktaGroupEveryone = "Everyone"
)

var oktaAPIScopes = []string{
	ScopeUserManage,
	ScopeUserRead,
	ScopeAppsManage,
	ScopeAppsRead,
	ScopeGroupsManage,
	ScopeGroupsRead,
	ScopeOrgsRead,
}

const (
	// ScopeUserManage allows to call okta API and manage users - create, update, delete deactive etc.
	ScopeUserManage = "okta.users.manage"
	// ScopeUserRead allows to read user information from Okta API.
	ScopeUserRead = "okta.users.read"
	// ScopeAppsManage allows to manage applications in Okta create (needed to auto-creation of SAML okta APP) or
	// manage Okta applications assignments.
	ScopeAppsManage = "okta.apps.manage"
	// ScopeAppsRead allows to read applications from Okta API.
	ScopeAppsRead = "okta.apps.read"
	// ScopeGroupsManage allows to manage groups in Okta
	ScopeGroupsManage = "okta.groups.manage"
	// ScopeGroupsRead allows to read groups from Okta API.
	ScopeGroupsRead = "okta.groups.read"
	// ScopeOrgsRead allows to read organization information from Okta API.
	ScopeOrgsRead = "okta.orgs.read"
)
