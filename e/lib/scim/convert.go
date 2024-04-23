package scim

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

const (
	attributeID         = "id"
	attributeExternalID = "externalId"
	attributeSchemas    = "schemas"
	attributeMeta       = "meta"
)

var reservedAttributeNames = [...]string{
	attributeID,
	attributeExternalID,
	attributeSchemas,
	attributeMeta,
}

// meta encodes the JSON wire format of the SCIM resource metadata.
type meta struct {
	ResourceType string     `mapstructure:"resourceType,omitempty"`
	Created      *time.Time `mapstructure:"created,omitempty"`
	LastModified *time.Time `mapstructure:"lastModified,omitempty"`
	Location     string     `mapstructure:"location,omitempty"`
	Version      string     `mapstructure:"version,omitempty"`
}

// attributeSet is an arbitrary mapping on names to structured values. Used as
// an intermediary format for parsing and formtting SCIM resources
type attributeSet map[string]interface{}

// resource represents the JSON wire format of a SCIM resource, which is
// essentially some metadata with a trailing collection of arbitrarily
// structured attributes
type resource struct {
	Schemas    []string `mapstructure:"schemas,omitempty"`
	ID         string   `mapstructure:"id,omitempty"`
	ExternalID string   `mapstructure:"externalId,omitempty"`
	Meta       meta     `mapstructure:"meta,omitempty"`

	Attributes attributeSet `mapstructure:",remain,omitempty"`
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

	value, err := time.Parse(time.RFC3339, data.(string))
	if err != nil {
		return nil, err
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

// UnmarshalResource parses a JSON stream into a valid SCIM resource object.
// We go through an intermediate attributeSet as we want to collect all of the
// top-level JSON fields that are not specifically part of the resource metadata
// and store them for later use, as these define the actual properties of the
// resource.
func UnmarshalResource(data io.Reader) (*scimpb.Resource, error) {
	decoder := json.NewDecoder(data)

	var attribs attributeSet
	if err := decoder.Decode(&attribs); err != nil {
		return nil, nil
	}

	var jsonFmt resource
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

	resources := make([]attributeSet, len(list.Resources))
	for i, r := range list.Resources {
		attribs, err := flattenResource(r)
		if err != nil {
			return nil, trace.Wrap(err, "flattening %s resource %s", r.Meta.ResourceType, r.Id)
		}
		resources[i] = attribs
	}

	body, err := json.Marshal(map[string]interface{}{
		"schemas":      []string{listResponseSchema},
		"totalResults": list.TotalResults,
		"itemsPerPage": list.ItemsPerPage,
		"startIndex":   list.StartIndex,
		"Resources":    resources,
	})
	if err != nil {
		return nil, trace.Wrap(err, "serializing resource list")
	}

	return body, nil
}

// flattenResource creates an attributeSet representing the supplied SCIM
// resource. We go through this intermediate flattening stage so that we cam
// merge the resource Attributes back into the top level of the structure
// before being serialized to JSON.
func flattenResource(res *scimpb.Resource) (attributeSet, error) {
	jsonFmt := resource{
		Schemas:    res.Schemas,
		ID:         res.Id,
		ExternalID: res.ExternalId,
		Meta: meta{
			ResourceType: res.Meta.ResourceType,
			Location:     res.Meta.Location,
			Version:      res.Meta.Version,
			Created:      maybeTime(res.Meta.Created),
			LastModified: maybeTime(res.Meta.Modified),
		},
	}

	// format the resource header as a nested set of attributes
	var attribs attributeSet
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
