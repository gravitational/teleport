package conv

import (
	"maps"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
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
		o.groups = groups
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
	addLabel(user, options.labels)
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
	resource := scimpb.Resource{
		Id:         user.GetName(),
		ExternalId: externalID,
		Meta: &scimpb.Meta{
			Created:      timestamppb.New(user.GetCreatedBy().Time),
			Version:      VersionAsETag(user.GetRevision()),
			ResourceType: common.ResourceTypeUser,
		},
	}
	attrs := map[string]any{common.UsernameAttribute: user.GetName()}
	if len(options.groups) > 0 {
		attrs[common.GroupsAttribute] = ToSCIMGroups(options.groups)
	}
	protoAttrs, err := structpb.NewStruct(attrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resource.Attributes = protoAttrs

	return &resource, nil
}

// UserResourceOption is a function that modifies the user resource options.
type UserResourceOption func(*userResourceOptions)

type userResourceOptions struct {
	groups       []string
	externalIDFn func(u types.User) string
	labels       map[string]string
	clock        clockwork.Clock
	connectorRef *types.ConnectorRef
}

type labelsGetterSetter interface {
	GetStaticLabels() map[string]string
	SetStaticLabels(map[string]string)
}

func addLabel(r labelsGetterSetter, m map[string]string) {
	labels := r.GetStaticLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	maps.Copy(labels, m)
	r.SetStaticLabels(labels)
}
