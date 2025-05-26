package conv

import (
	"maps"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

// AccessListToResource encodes a teleport AccessList and its associated
// AccessListMember records into an RFC 7543-compliant SCIM group resource
func AccessListToResource(accessList *accesslist.AccessList, members []*accesslist.AccessListMember) (*scimpb.Resource, error) {
	memberResources := make([]any, 0, len(members))
	for _, m := range members {
		memberResources = append(memberResources,
			map[string]any{"display": m.GetName(), "value": m.GetName()})
	}

	groupAttrs := map[string]any{
		"displayName": accessList.Spec.Title,
		"members":     memberResources,
	}

	attrs, err := structpb.NewStruct(groupAttrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resource := &scimpb.Resource{
		Id: accessList.GetName(),
		Meta: &scimpb.Meta{
			ResourceType: common.ResourceTypeGroup,
			Version:      accessList.GetRevision(),
		},
		Attributes: attrs,
	}
	return resource, nil
}

// WithClock sets the clock to be used for time-based operations
func WithClock(clock clockwork.Clock) AccessListToResourceFunc {
	return func(o *accessListToResourceOptions) {
		o.clock = clock
	}
}

// WithGrants sets the grants to be applied to the access list
func WithGrants(grants accesslist.Grants) AccessListToResourceFunc {
	return func(o *accessListToResourceOptions) {
		o.grants = grants
	}
}

// WithAccessListLabels sets the labels to be applied to the access list
func WithAccessListLabels(labels map[string]string) AccessListToResourceFunc {
	return func(o *accessListToResourceOptions) {
		o.accessListLabels = labels
	}
}

// AccessListFromResource decodes an SCIM group resource into a Teleport Access List.
func AccessListFromResource(r *scimpb.Resource, opts ...AccessListToResourceFunc) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	options := accessListToResourceOptions{
		clock: clockwork.NewRealClock(),
	}
	for _, opt := range opts {
		opt(&options)
	}
	group, err := decodeGroupResource(r.Attributes.AsMap())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	acl := &accesslist.AccessList{
		ResourceHeader: header.ResourceHeader{
			Metadata: header.Metadata{
				Name:     r.Id,
				Labels:   maps.Clone(options.accessListLabels),
				Revision: r.GetMeta().GetVersion(),
			},
		},
		Spec: accesslist.Spec{
			Title:  group.DisplayName,
			Grants: options.grants,
		},
	}

	members := make([]*accesslist.AccessListMember, len(group.Members))
	for i, m := range group.Members {
		newMember := &accesslist.AccessListMember{
			ResourceHeader: header.ResourceHeader{
				Metadata: header.Metadata{
					Name:   m.Value,
					Labels: maps.Clone(options.accessListLabels),
				},
			},
			Spec: accesslist.AccessListMemberSpec{
				AccessList: acl.GetName(),
				Name:       m.Value,
				Joined:     options.clock.Now(),
			},
		}
		members[i] = newMember
	}
	return acl, members, nil
}

// AccessListToResourceFunc is a function that modifies the options
type AccessListToResourceFunc func(*accessListToResourceOptions)

type accessListToResourceOptions struct {
	clock            clockwork.Clock
	accessListLabels map[string]string
	grants           accesslist.Grants
}

// member holds a SCIM group membership record as per RFC 7643 Section 4.2
type member struct {
	Value   string `mapstructure:"value"`
	Display string `mapstructure:"display"`
}

// groupResource uses holds a parsed representation of a SCIM group resource,
// as per RFC 7643 Section 4.2
type groupResource struct {
	DisplayName string   `mapstructure:"displayName"`
	Members     []member `mapstructure:"members"`
}

// decodeGroupResource parses a SCIM group resource using `mapstructure`
func decodeGroupResource(attributes map[string]any) (groupResource, error) {
	var group groupResource
	if err := mapstructure.Decode(attributes, &group); err != nil {
		return groupResource{}, trace.Wrap(err)
	}
	return group, nil
}

func getAttr(attrs map[string]any, key string) (string, error) {
	untypedValue, ok := attrs[key]
	if !ok {
		return "", trace.BadParameter("missing required attribute %s", key)
	}
	value, ok := untypedValue.(string)
	if !ok {
		return "", trace.BadParameter("invalid attribute type %T", untypedValue)
	}
	return value, nil
}
