package scimsdk

import (
	"encoding/json"
	"io"
	"maps"
	"reflect"
	"strconv"
	"time"

	scimSchema "github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	scimconv "github.com/gravitational/teleport/e/lib/scim/conv"
)

const (
	// ResourceTypeUser indicates that an SCIM resource is a user, as per RFC 7643
	ResourceTypeUser = "User"

	// ResourceTypeGroup indicates that an SCIM resource is a group, as per RFC 7643
	ResourceTypeGroup = "Group"
)

// UnmarshalAttributeSet parses the data in the supplied stream as JSON and marshals it
// into an arbitrary [AttributeSet]. From there it can be decoded into a SCIM resource.
func UnmarshalAttributeSet(data io.Reader) (AttributeSet, error) {
	var attribs AttributeSet
	if err := json.NewDecoder(data).Decode(&attribs); err != nil {
		return nil, trace.Wrap(err)
	}
	return attribs, nil
}

// UnmarshalResourceHeader parses a JSON stream into a valid SCIM resource object.
// We go through an intermediate attributeSet as we want to collect all of the
// top-level JSON fields that are not specifically part of the resource metadata
// and store them for later use, as these define the actual properties of the
// resource.
func UnmarshalResourceHeader(data io.Reader) (*Resource, error) {
	decoder := json.NewDecoder(data)

	var attribs AttributeSet
	if err := decoder.Decode(&attribs); err != nil {
		return nil, trace.Wrap(err)
	}

	jsonFmt, err := DecodeResourceHeader(attribs)
	if err != nil {
		return nil, trace.Wrap(err, "decoding resource header")
	}

	return jsonFmt, nil
}

// DecodeResource converts an arbitrary AttributeSet received from a client into a
// [scimpb.Resource] suitable for sending to the SCIM gRPCservice for prcessing.
func DecodeResource(attributes AttributeSet) (*scimpb.Resource, error) {
	header, err := DecodeResourceHeader(attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dstAttribs, err := structpb.NewStruct(header.Attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dst := scimpb.Resource_builder{
		Schemas:    header.Schemas,
		Id:         header.ID,
		ExternalId: header.ExternalID,
		Meta:       convertMetadata(header.Meta),
		Attributes: dstAttribs,
	}.Build()

	return dst, nil
}

// UnmarshalResource parses a JSON stream into a valid SCIM resource object.
// We go through an intermediate attributeSet as we want to collect all of the
// top-level JSON fields that are not specifically part of the resource metadata
// and store them for later use, as these define the actual properties of the
// resource.
func UnmarshalResource(data io.Reader) (*scimpb.Resource, error) {
	attribs, err := UnmarshalAttributeSet(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dst, err := DecodeResource(attribs)
	if err != nil {
		return nil, trace.Wrap(err, "un-marshaling SCIM resource header")
	}
	return dst, nil
}

func convertMetadata(src *Metadata) *scimpb.Meta {
	if src == nil {
		return nil
	}
	return scimpb.Meta_builder{
		ResourceType: src.ResourceType,
		Location:     src.Location,
		Version:      src.Version,
		Created:      maybeTimestamp(src.Created),
		Modified:     maybeTimestamp(src.LastModified),
	}.Build()
}

// MarshalResourceList flattens and formats a collection of resources, wrapping
// them in a valid SCIM list response before serializing them to JSON.
func MarshalResourceList(list *scimpb.ResourceList) ([]byte, error) {
	const (
		listResponseSchema = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	)

	resources := make([]AttributeSet, len(list.GetResources()))
	for i, r := range list.GetResources() {
		attribs, err := FlattenResource(r)
		if err != nil {
			return nil, trace.Wrap(err, "flattening %s resource %s", r.GetMeta().GetResourceType(), r.GetId())
		}
		resources[i] = attribs
	}

	body, err := json.Marshal(ListResponse{
		Schemas:      []string{listResponseSchema},
		TotalResults: list.GetTotalResults(),
		ItemsPerPage: list.GetItemsPerPage(),
		StartIndex:   list.GetStartIndex(),
		Resources:    resources,
	})
	if err != nil {
		return nil, trace.Wrap(err, "serializing resource list")
	}

	return body, nil
}

// stringToDateTimeHook parses an RFC3339 timestamp string into GO time.Time.
// For use with mapstructure.Decode()
func stringToDateTimeHook(from reflect.Type, to reflect.Type, data any) (any, error) {
	if from.Kind() != reflect.String {
		return data, nil
	}
	if to != reflect.TypeOf(&time.Time{}) {
		return data, nil
	}

	s, ok := data.(string)
	if !ok {
		return nil, trace.BadParameter("expected string, got %T", data)
	}
	value, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &value, nil
}

// maybeTimestamp translates a Go time.Time to a protobuf Timestamp if the
// supplied value is non-nil. Otherwise, the nil passes through unmolested.
func maybeTimestamp(src *time.Time) *timestamppb.Timestamp {
	if src == nil {
		return nil
	}
	return timestamppb.New(*src)
}

// maybeTime translates a protobuf Timestamp to a Go time if the supplied value
// is non-nil. Otherwise, the nil passes through unmolested
func maybeTime(src *timestamppb.Timestamp) *time.Time {
	if src == nil {
		return nil
	}
	dst := src.AsTime()
	return &dst
}

// DecodeResourceHeader converts a flat attribute set into a SCIM resource object.
func DecodeResourceHeader(attribs AttributeSet) (*Resource, error) {
	var jsonFmt Resource
	mapDecoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &jsonFmt,
		DecodeHook: stringToDateTimeHook,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := mapDecoder.Decode(attribs); err != nil {
		return nil, trace.Wrap(err)
	}

	return &jsonFmt, nil
}

// FlattenResource creates an attributeSet representing the supplied SCIM
// resource. We go through this intermediate flattening stage so that we can
// merge the resource Attributes back into the top level of the structure
// before being serialized to JSON.
func FlattenResource(res *scimpb.Resource) (AttributeSet, error) {
	headerFmt := Resource{
		Schemas:    res.GetSchemas(),
		ID:         res.GetId(),
		ExternalID: res.GetExternalId(),
		Meta: &Metadata{
			ResourceType: res.GetMeta().GetResourceType(),
			Location:     res.GetMeta().GetLocation(),
			Version:      res.GetMeta().GetVersion(),
			Created:      maybeTime(res.GetMeta().GetCreated()),
			LastModified: maybeTime(res.GetMeta().GetModified()),
		},
	}

	// format the resource header as a nested set of attributes
	var dst AttributeSet
	if err := mapstructure.Decode(&headerFmt, &dst); err != nil {
		return nil, trace.Wrap(err)
	}

	// Copy the resource-specific resources into the toplevel of the
	// JSON struct, minus anything that would break the SCIM schema
	resourceAttribs := res.GetAttributes().AsMap()
	for _, k := range reservedAttributeNames {
		delete(resourceAttribs, k)
	}
	maps.Copy(dst, res.GetAttributes().AsMap())

	return dst, nil
}

// MarshalResource flattens and formats a SCIM resource object into JSON.
func MarshalResource(res *scimpb.Resource) ([]byte, error) {
	attribs, err := FlattenResource(res)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling SCIM resource")
	}

	// Format the lot as JSON and return to the caller
	data, err := json.Marshal(&attribs)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling SCIM resource")
	}

	return data, nil
}

// UserActiveState indicates whether a user is considered "active" or not, as
// per RFC7643, section 4.4.1. The interpretation of "active" is loosely defined
// and varies between SCIM servers (for example, Okta uses the "active" field
// to indicate that it is taking over provisioning for an existing in the
// downstream service), but in general indicates whether a user should be
// enabled or disabled.
type UserActiveState bool

const (
	// UserActive indicates that the user should be enabled in the target
	// system
	UserActive UserActiveState = true

	// UserInactive indicates that the user should be enabled in the target
	// system
	UserInactive UserActiveState = false
)

type userOption func(*User)

func WithUserID(id string) userOption {
	return func(u *User) {
		u.ID = id
	}
}

func WithActiveState(state UserActiveState) userOption {
	return func(u *User) {
		u.Active = bool(state)
	}
}

// ToUser generates a SCIM user resource from the supplied Teleport User
func ToUser(user types.User, options ...userOption) *User {
	// TODO(tcsc): Work out how to synthesize sensible values for required
	//             attributes. I'm envisioning having some kind of config that
	//             maps Teleport User traits to SCIM user-schema attributes,
	//             possibly with fallback through multiple options.
	//
	//             This current implementation passes specific attributes from Okta
	//             through tp AWS, as this is the initial use case, but users will
	//             not always be sourced from Okta, and a more general solution
	//             is required.
	u := &User{
		Schemas: []string{scimSchema.UserSchema},
		Meta: &Metadata{
			ResourceType: ResourceTypeUser,
			Version:      scimconv.VersionAsETag(user.GetVersion()),
		},
		ExternalID:  user.GetName(),
		UserName:    user.GetName(),
		DisplayName: traitOrDefault(user, "okta/displayName", user.GetName()),
		Active:      true,
		Name: &Name{
			GivenName:  traitOrDefault(user, "okta/givenName", "-"),
			FamilyName: traitOrDefault(user, "okta/familyName", "-"),
		},
	}
	for _, opt := range options {
		opt(u)
	}
	return u
}

// traitOrDefault returns the first defined value for the given trait, or the
// supplied default if the requested trait is mission or empty
func traitOrDefault(user types.User, trait string, defaultValue string) string {
	values, ok := user.GetTraits()[trait]
	if !ok || len(values) == 0 {
		return defaultValue
	}

	return values[0]
}

type groupOption func(*Group)

func WithGroupID(id string) groupOption {
	return func(grp *Group) {
		grp.ID = id
	}
}

// ToGroup generates a SCIM group resource from the supplied Teleport Access
// List. Note that the returned Group dows not include a member list.
func ToGroup(acl *accesslist.AccessList, options ...groupOption) *Group {
	g := &Group{
		Meta: &Metadata{
			ResourceType: ResourceTypeGroup,
			Version:      strconv.Quote(acl.Version),
		},
		Schemas:     []string{scimSchema.GroupSchema},
		DisplayName: acl.Spec.Title,
	}
	for _, opt := range options {
		opt(g)
	}
	return g
}
