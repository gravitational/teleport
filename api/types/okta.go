package types

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/trace"
)

// OktaApplication represents an Okta application.
type OktaApplication interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetDescription returns the Okta app description.
	GetDescription() string
	// GetUsers returns the Okta app users.
	GetUsers() []string
	// GetGroups returns the Okta app groups.
	GetGroups() []string
	// GetAppLinks returns the Okta app links.
	GetAppLinks() []OktaAppLink
}

// NewOktaApplicationV1 creates a new Okta application resource.
func NewOktaApplicationV1(meta Metadata, spec OktaApplicationSpecV1) (*OktaApplicationV1, error) {
	oktaApp := &OktaApplicationV1{
		Metadata: meta,
		Spec:     spec,
	}
	if err := oktaApp.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return oktaApp, nil
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (o *OktaApplicationV1) CheckAndSetDefaults() error {
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if o.Kind == "" {
		o.Kind = "oktaapps"
	}
	if o.Version == "" {
		o.Version = "v1"
	}
	if o.Spec.Id == "" {
		return trace.BadParameter("Id is missing")
	}
	if o.Spec.Users == nil {
		o.Spec.Users = make([]string, 0)
	}
	if o.Spec.Groups == nil {
		o.Spec.Groups = make([]string, 0)
	}
	if len(o.Spec.AppLinks) == 0 {
		return trace.BadParameter("AppLinks must be greater than 0")
	}

	return nil
}

// GetVersion returns the app resource version.
func (o *OktaApplicationV1) GetVersion() string {
	return o.Version
}

// GetKind returns the app resource kind.
func (o *OktaApplicationV1) GetKind() string {
	return o.Kind
}

// GetSubKind returns the app resource subkind.
func (o *OktaApplicationV1) GetSubKind() string {
	return o.SubKind
}

// SetSubKind sets the app resource subkind.
func (o *OktaApplicationV1) SetSubKind(sk string) {
	o.SubKind = sk
}

// GetResourceID returns the app resource ID.
func (o *OktaApplicationV1) GetResourceID() int64 {
	return o.Metadata.ID
}

// SetResourceID sets the app resource ID.
func (o *OktaApplicationV1) SetResourceID(id int64) {
	o.Metadata.ID = id
}

// GetMetadata returns the app resource metadata.
func (o *OktaApplicationV1) GetMetadata() Metadata {
	return o.Metadata
}

// Origin returns the origin value of the resource.
func (o *OktaApplicationV1) Origin() string {
	return o.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (o *OktaApplicationV1) SetOrigin(origin string) {
	o.Metadata.SetOrigin(origin)
}

// GetNamespace returns the app resource namespace.
func (o *OktaApplicationV1) GetNamespace() string {
	return o.Metadata.Namespace
}

// SetExpiry sets the app resource expiration time.
func (o *OktaApplicationV1) SetExpiry(expiry time.Time) {
	o.Metadata.SetExpiry(expiry)
}

// Expiry returns the app resource expiration time.
func (o *OktaApplicationV1) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// GetName returns the app resource name.
func (o *OktaApplicationV1) GetName() string {
	return o.Metadata.Name
}

// SetName sets the app resource name.
func (o *OktaApplicationV1) SetName(name string) {
	o.Metadata.Name = name
}

// GetAllLabels returns the app combined static and dynamic labels.
func (o *OktaApplicationV1) GetAllLabels() map[string]string {
	return o.Metadata.Labels
}

func (o *OktaApplicationV1) GetStaticLabels() map[string]string {
	return o.Metadata.Labels
}

func (o *OktaApplicationV1) SetStaticLabels(sl map[string]string) {
	o.Metadata.Labels = sl
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (o *OktaApplicationV1) GetDescription() string {
	return o.Metadata.Description
}

// GetUsers returns the list of users assigned to the application.
func (o *OktaApplicationV1) GetUsers() []string {
	users := make([]string, len(o.Spec.Users))
	copy(users, o.Spec.Users)
	return users
}

// GetGroups returns the list of groups assigned to the application.
func (o *OktaApplicationV1) GetGroups() []string {
	groups := make([]string, len(o.Spec.Groups))
	copy(groups, o.Spec.Groups)
	return groups
}

// GetAppLinks returns the list of app links associated with the application.
func (o *OktaApplicationV1) GetAppLinks() []OktaAppLink {
	appLinks := make([]OktaAppLink, len(o.Spec.AppLinks))
	for i, appLink := range o.Spec.AppLinks {
		appLinks[i] = appLink
	}
	return appLinks
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (o *OktaApplicationV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(o.GetAllLabels()), o.GetName(), o.GetDescription())
	return MatchSearch(fieldVals, values, nil)
}

func (o *OktaApplicationV1) String() string {
	builder := strings.Builder{}
	builder.WriteString("OktaApplicationV1 {\n")

	builder.WriteString(o.Metadata.String() + "\n")

	if len(o.Spec.Users) > 0 {
		builder.WriteString("  users: {\n")
		for _, user := range o.Spec.Users {
			builder.WriteString(fmt.Sprintf("    %s,\n", user))
		}
		builder.WriteString("  }\n")
	}
	if len(o.Spec.Groups) > 0 {
		builder.WriteString("  groups: {\n")
		for _, group := range o.Spec.Groups {
			builder.WriteString(fmt.Sprintf("    %s,\n", group))
		}
		builder.WriteString("  }\n")
	}
	if len(o.Spec.AppLinks) > 0 {
		builder.WriteString("  app_links: {\n")
		for _, appLink := range o.Spec.AppLinks {
			builder.WriteString(fmt.Sprintf("    %s,\n", appLink))
		}
		builder.WriteString("  }\n")
	}

	return builder.String()
}

// OktaGroup represents an Okta group.
type OktaGroup interface {
	// ResourceWithLabels provides common resource methods.
	ResourceWithLabels
	// GetApplications returns the Okta applications in the group.
	GetApplications() []string
	// GetUsers returns the Okta users in the group.
	GetUsers() []string
}

// NewOktaGroupV1 creates a new Okta group resource.
func NewOktaGroupV1(meta Metadata, spec OktaGroupSpecV1) (*OktaGroupV1, error) {
	oktaGroup := &OktaGroupV1{
		Metadata: meta,
		Spec:     spec,
	}
	if err := oktaGroup.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return oktaGroup, nil
}

// CheckAndSetDefaults checks and sets default values for any missing fields.
func (o *OktaGroupV1) CheckAndSetDefaults() error {
	if err := o.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if o.Kind == "" {
		o.Kind = "oktagroups"
	}
	if o.Version == "" {
		o.Version = "v1"
	}
	if o.Spec.Applications == nil {
		o.Spec.Applications = make([]string, 0)
	}
	if o.Spec.Users == nil {
		o.Spec.Users = make([]string, 0)
	}

	return nil
}

// GetVersion returns the app resource version.
func (o *OktaGroupV1) GetVersion() string {
	return o.Version
}

// GetKind returns the app resource kind.
func (o *OktaGroupV1) GetKind() string {
	return o.Kind
}

// GetSubKind returns the app resource subkind.
func (o *OktaGroupV1) GetSubKind() string {
	return o.SubKind
}

// SetSubKind sets the app resource subkind.
func (o *OktaGroupV1) SetSubKind(sk string) {
	o.SubKind = sk
}

// GetResourceID returns the app resource ID.
func (o *OktaGroupV1) GetResourceID() int64 {
	return o.Metadata.ID
}

// SetResourceID sets the app resource ID.
func (o *OktaGroupV1) SetResourceID(id int64) {
	o.Metadata.ID = id
}

// GetMetadata returns the app resource metadata.
func (o *OktaGroupV1) GetMetadata() Metadata {
	return o.Metadata
}

// Origin returns the origin value of the resource.
func (o *OktaGroupV1) Origin() string {
	return o.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (o *OktaGroupV1) SetOrigin(origin string) {
	o.Metadata.SetOrigin(origin)
}

// GetNamespace returns the app resource namespace.
func (o *OktaGroupV1) GetNamespace() string {
	return o.Metadata.Namespace
}

// SetExpiry sets the app resource expiration time.
func (o *OktaGroupV1) SetExpiry(expiry time.Time) {
	o.Metadata.SetExpiry(expiry)
}

// Expiry returns the app resource expiration time.
func (o *OktaGroupV1) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// GetName returns the app resource name.
func (o *OktaGroupV1) GetName() string {
	return o.Metadata.Name
}

// SetName sets the app resource name.
func (o *OktaGroupV1) SetName(name string) {
	o.Metadata.Name = name
}

// GetAllLabels returns the app combined static and dynamic labels.
func (o *OktaGroupV1) GetAllLabels() map[string]string {
	return o.Metadata.Labels
}

func (o *OktaGroupV1) GetStaticLabels() map[string]string {
	return o.Metadata.Labels
}

func (o *OktaGroupV1) SetStaticLabels(sl map[string]string) {
	o.Metadata.Labels = sl
}

func (o *OktaGroupV1) GetApplications() []string {
	applications := make([]string, len(o.Spec.Applications))
	copy(applications, o.Spec.Applications)
	return applications
}

func (o *OktaGroupV1) GetUsers() []string {
	users := make([]string, len(o.Spec.Users))
	copy(users, o.Spec.Users)
	return users
}

// MatchSearch goes through select field values and tries to
// match against the list of search values.
func (o *OktaGroupV1) MatchSearch(values []string) bool {
	fieldVals := append(utils.MapToStrings(o.GetAllLabels()), o.GetName())
	return MatchSearch(fieldVals, values, nil)
}

func (o *OktaGroupV1) String() string {
	builder := strings.Builder{}
	builder.WriteString("OktaGroupV1 {\n")

	builder.WriteString(o.Metadata.String())

	if len(o.Spec.Applications) > 0 {
		builder.WriteString("  applications: {\n")
		for _, application := range o.Spec.Applications {
			builder.WriteString(fmt.Sprintf("    %s,\n", application))
		}
		builder.WriteString("  }\n")
	}
	if len(o.Spec.Users) > 0 {
		builder.WriteString("  users: {\n")
		for _, user := range o.Spec.Users {
			builder.WriteString(fmt.Sprintf("    %s,\n", user))
		}
		builder.WriteString("  }\n")
	}

	return builder.String()
}

type OktaAppLink interface {
	GetName() string
	GetUri() string
}

func (o *OktaAppLinkV1) GetName() string {
	return o.Name
}

func (o *OktaAppLinkV1) GetUri() string {
	return o.Uri
}
