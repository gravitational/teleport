package scimsdk

import (
	"encoding/json"
	"io"
	"maps"
	"reflect"
	"time"

	"github.com/gravitational/trace"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

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

// UnmarshalResource parses a JSON stream into a valid SCIM resource object.
// We go through an intermediate attributeSet as we want to collect all of the
// top-level JSON fields that are not specifically part of the resource metadata
// and store them for later use, as these define the actual properties of the
// resource.
func UnmarshalResource(data io.Reader) (*scimpb.Resource, error) {
	var jsonFmt *Resource
	var err error

	jsonFmt, err = UnmarshalResourceHeader(data)
	if err != nil {
		return nil, trace.Wrap(err, "un-marshaling SCIM resource header")
	}

	dstAttribs, err := structpb.NewStruct(jsonFmt.Attributes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dst := &scimpb.Resource{
		Schemas:    jsonFmt.Schemas,
		Id:         jsonFmt.ID,
		ExternalId: jsonFmt.ExternalID,
		Meta: &scimpb.Meta{
			ResourceType: jsonFmt.Meta.ResourceType,
			Location:     jsonFmt.Meta.Location,
			Version:      jsonFmt.Meta.Version,
			Created:      maybeTimestamp(jsonFmt.Meta.Created),
			Modified:     maybeTimestamp(jsonFmt.Meta.LastModified),
		},
		Attributes: dstAttribs,
	}

	return dst, nil
}

// MarshalResourceList flattens and formats a collection of resources, wrapping
// them in a valid SCIM list response before serializing them to JSON.
func MarshalResourceList(list *scimpb.ResourceList) ([]byte, error) {
	const (
		listResponseSchema = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	)

	resources := make([]AttributeSet, len(list.Resources))
	for i, r := range list.Resources {
		attribs, err := flattenResource(r)
		if err != nil {
			return nil, trace.Wrap(err, "flattening %s resource %s", r.Meta.ResourceType, r.Id)
		}
		resources[i] = attribs
	}

	body, err := json.Marshal(ListResponse{
		Schemas:      []string{listResponseSchema},
		TotalResults: list.TotalResults,
		ItemsPerPage: list.ItemsPerPage,
		StartIndex:   list.StartIndex,
		Resources:    resources,
	})
	if err != nil {
		return nil, trace.Wrap(err, "serializing resource list")
	}

	return body, nil
}

// stringToDateTimeHook parses an RFC3339 timestamp string into GO time.Time.
// For use with mapstructure.Decode()
func stringToDateTimeHook(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
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

// flattenResource creates an attributeSet representing the supplied SCIM
// resource. We go through this intermediate flattening stage so that we cam
// merge the resource Attributes back into the top level of the structure
// before being serialized to JSON.
func flattenResource(res *scimpb.Resource) (AttributeSet, error) {
	jsonFmt := Resource{
		Schemas:    res.Schemas,
		ID:         res.Id,
		ExternalID: res.ExternalId,
		Meta: &Metadata{
			ResourceType: res.Meta.ResourceType,
			Location:     res.Meta.Location,
			Version:      res.Meta.Version,
			Created:      maybeTime(res.Meta.Created),
			LastModified: maybeTime(res.Meta.Modified),
		},
	}

	// format the resource header as a nested set of attributes
	var attribs AttributeSet
	if err := mapstructure.Decode(&jsonFmt, &attribs); err != nil {
		return nil, trace.Wrap(err)
	}

	// Copy the resource-specific resources into the toplevel of the
	// JSON struct, minus anything that would break the SCIM schema
	resourceAttribs := res.Attributes.AsMap()
	for _, k := range reservedAttributeNames {
		delete(resourceAttribs, k)
	}
	maps.Copy(attribs, res.Attributes.AsMap())

	return attribs, nil
}

// MarshalResource flattens and formats a SCIM resource object into a JSON.
func MarshalResource(res *scimpb.Resource) ([]byte, error) {
	attribs, err := flattenResource(res)
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
