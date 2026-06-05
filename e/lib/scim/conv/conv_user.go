package conv

import (
	"encoding/json"
	"maps"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

// WithUserOptionClock sets the clock to be used for time-based operations.
func WithUserOptionClock(c clockwork.Clock) UserResourceOption {
	return func(o *userResourceOptions) {
		o.clock = c
	}
}

// WithConnectorRef sets the connector reference for the user resource.
func WithConnectorRef(connectorRef *types.ConnectorRef) UserResourceOption {
	return func(o *userResourceOptions) {
		o.connectorRef = connectorRef
	}
}

// WithLabels sets the labels for the user resource.
func WithLabels(labels map[string]string) UserResourceOption {
	return func(o *userResourceOptions) {
		o.labels = labels
	}
}

// WithExternalIDFunc sets the external ID for the user resource.
func WithExternalIDFunc(f func(u types.User) string) UserResourceOption {
	return func(o *userResourceOptions) {
		o.externalIDFn = f
	}
}

// WithGroupsAttr sets the groups SCIM attribute for the user resource.
func WithGroupsAttr(groups []string) UserResourceOption {
	return func(o *userResourceOptions) {
		o.groupsVal = &groupsVal{groups: groups}
	}
}

// UserFromResource converts an SCIM resource to a Teleport user.
func UserFromResource(r *scimpb.Resource, opts ...UserResourceOption) (types.User, error) {
	if r.GetAttributes() == nil {
		return nil, trace.BadParameter("missing resource attributes")
	}
	options := userResourceOptions{
		clock: clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(&options)
	}
	scimAttribs := r.GetAttributes().AsMap()
	username := r.GetId()
	if username == "" {
		var err error
		username, err = getAttr(scimAttribs, common.UsernameAttribute)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.SetCreatedBy(types.CreatedBy{
		User:      types.UserRef{Name: teleport.UserSystem},
		Time:      options.clock.Now().UTC(),
		Connector: options.connectorRef,
	})
	user.SetRevision(ResourceVersion(r))
	if err := SetSCIMAttrsInUserLabel(user, r); err != nil {
		return nil, trace.Wrap(err)
	}
	addLabels(user, options.labels)
	return user, nil
}

// UserToResource converts a Teleport user to an SCIM resource.
func UserToResource(user types.User, opts ...UserResourceOption) (*scimpb.Resource, error) {
	options := userResourceOptions{
		clock: clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(&options)
	}
	var externalID string
	if options.externalIDFn != nil {
		externalID = options.externalIDFn(user)
	}
	resource := scimpb.Resource_builder{
		Id:         user.GetName(),
		ExternalId: externalID,
		Meta: scimpb.Meta_builder{
			Created:      timestamppb.New(user.GetCreatedBy().Time),
			Version:      VersionAsETag(user.GetRevision()),
			ResourceType: common.ResourceTypeUser,
		}.Build(),
	}.Build()
	if err := setSCIMAttrsInResource(resource, user, options.groupsVal); err != nil {
		return nil, trace.Wrap(err, "setting SCIM resource attributes from user label")
	}

	return resource, nil
}

// SetSCIMAttrsInUserLabel sets "teleport.internal/scim-attrs" label to JSON formatted SCIM
// attributes present in the provided SCIM resource. This information is used to provide the same
// attributes in any subsequent SCIM response resource.
func SetSCIMAttrsInUserLabel(u types.User, r *scimpb.Resource) error {
	// 100 KB is 1/4 of DynamoDB 400KB per-item limit. In case of Okta we also need roughly as
	// much for traits from the app profile and traits from SAML attributes.
	return setSCIMAttrsInUserLabel(u, r, 100*1024)
}

// setSCIMAttrsInUserLabel is extracted for testing.
func setSCIMAttrsInUserLabel(u types.User, r *scimpb.Resource, maxSCIMAttrsLabelSize int) error {
	fields := r.GetAttributes().GetFields()
	if fields == nil {
		return nil
	}

	// delete password field if Okta sets it, we don't want to leak it
	delete(fields, common.PasswordAttribute)

	// groups are derived from access list membership, not from the SCIM request body.
	// Storing them in the label doesn't make sense because they are always dynamically calculated.
	delete(fields, common.GroupsAttribute)

	scimAttrsJSON, err := json.Marshal(fields)
	if err != nil {
		return trace.Wrap(err, "JSON encoding SCIM attributes")
	}
	// restrict the label value size to maxSCIMAttrsLabelSize
	if len(scimAttrsJSON) > maxSCIMAttrsLabelSize {
		return trace.BadParameter("teleport.internal/scim-attrs label value too large (max = [%d], actual = [%d]); reduce the number or trim the values of the attributes in your Okta SCIM app profile", maxSCIMAttrsLabelSize, len(scimAttrsJSON))
	}

	addLabel(u, eteleport.SCIMAttrsLabel, string(scimAttrsJSON))
	return nil
}

// SetSCIMAttrsLabel sets SCIM resource attributes read  from JSON formatted
// "teleport.internal/scim-attrs" user label. The only exception is the "userName" attribute which
// is always set to the Teleport user's name.
func setSCIMAttrsInResource(r *scimpb.Resource, u types.User, groupsVal *groupsVal) error {
	var attrs map[string]any
	if attrsJSON, _ := u.GetLabel(eteleport.SCIMAttrsLabel); attrsJSON != "" {
		if err := json.Unmarshal([]byte(attrsJSON), &attrs); err != nil {
			return trace.Wrap(err, "JSON encoding user's SCIM attributes")
		}
		attrs[common.UsernameAttribute] = u.GetName()
	} else {
		attrs = map[string]any{common.UsernameAttribute: u.GetName()}
	}
	// Always remove stale groups from the label - groups attribute is manage by SCIM Service Provider - Teleport
	// where the groups are calculated dynamically based user membership state.
	delete(attrs, common.GroupsAttribute)

	if groupsVal != nil {
		attrs[common.GroupsAttribute] = ToSCIMGroups(groupsVal.groups)
	}
	attrsProto, err := structpb.NewStruct(attrs)
	if err != nil {
		return trace.Wrap(err, "creating new protobuf struct")
	}
	r.SetAttributes(attrsProto)
	return nil
}

// UserResourceOption is a function that modifies the user resource options.
type UserResourceOption func(*userResourceOptions)

type userResourceOptions struct {
	// groupsVal is a pointer to allow to distinguish
	// empty settings for no settings groups attributes.
	groupsVal    *groupsVal
	externalIDFn func(u types.User) string
	labels       map[string]string
	clock        clockwork.Clock
	connectorRef *types.ConnectorRef
}

type groupsVal struct {
	groups []string
}

type labelsGetterSetter interface {
	GetStaticLabels() map[string]string
	SetStaticLabels(map[string]string)
}

func addLabel(r labelsGetterSetter, k, v string) {
	labels := r.GetStaticLabels()
	if labels == nil {
		labels = map[string]string{k: v}
	} else {
		labels[k] = v
	}
	r.SetStaticLabels(labels)
}

func addLabels(r labelsGetterSetter, m map[string]string) {
	labels := r.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string, len(m))
	}
	maps.Copy(labels, m)
	r.SetStaticLabels(labels)
}
