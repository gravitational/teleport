package services

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

// UnifiedResource153 is a type constraint that requires a type to implement
// the Resource153 interface AND provides a Clone method.
type UnifiedResource153 interface {
	types.Resource153
	CloneResource() types.Resource153
}

// UnifiedResource153Adapter is a wrapper around a newer, RFD153-style resource
// that provides the newer style resources with the interfaces required for use
// with the Unified resource cache.
type UnifiedResource153Adapter struct {
	Inner UnifiedResource153
}

// WrapUnifiedResource153 wraps a RFD153-style resource in a type that implements
// the interfaces required for use with the Unified Resource Cache
func WrapUnifiedResource153(r UnifiedResource153) UnifiedResource153Adapter {
	return UnifiedResource153Adapter{Inner: r}
}

// Unwrap pulls the underlying resource out of the wrapper.
func (r UnifiedResource153Adapter) Unwrap() UnifiedResource153 {
	return r.Inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r UnifiedResource153Adapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Inner)
}

// Expiry maps the RFD153 metadata expiry time (which is a protobuf timestamp)
// to the older style resource Expiry (which is a Go time.Time).
func (r UnifiedResource153Adapter) Expiry() time.Time {
	expires := r.Inner.GetMetadata().Expires
	// return zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	if expires == nil {
		return time.Time{}
	}

	return expires.AsTime()
}

func (r UnifiedResource153Adapter) GetKind() string {
	return r.Inner.GetKind()
}

func (r UnifiedResource153Adapter) GetMetadata() types.Metadata {
	md := r.Inner.GetMetadata()

	// use zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	expires := md.Expires.AsTime()
	if md.Expires == nil {
		expires = time.Time{}
	}

	return types.Metadata{
		Name:        md.Name,
		Namespace:   md.Namespace,
		Description: md.Description,
		Labels:      md.Labels,
		Expires:     &expires,
		Revision:    md.Revision,
	}
}

func (r UnifiedResource153Adapter) GetName() string {
	return r.Inner.GetMetadata().Name
}

func (r UnifiedResource153Adapter) GetRevision() string {
	return r.Inner.GetMetadata().Revision
}

func (r UnifiedResource153Adapter) GetSubKind() string {
	return r.Inner.GetSubKind()
}

func (r UnifiedResource153Adapter) GetVersion() string {
	return r.Inner.GetVersion()
}

func (r UnifiedResource153Adapter) SetExpiry(t time.Time) {
	r.Inner.GetMetadata().Expires = timestamppb.New(t)
}

func (r UnifiedResource153Adapter) SetName(name string) {
	r.Inner.GetMetadata().Name = name
}

func (r UnifiedResource153Adapter) SetRevision(rev string) {
	r.Inner.GetMetadata().Revision = rev
}

func (r UnifiedResource153Adapter) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}

func (r UnifiedResource153Adapter) Origin() string {
	m := r.Inner.GetMetadata()
	if m == nil {
		return ""
	}
	return m.Labels[types.OriginLabel]
}

func (r UnifiedResource153Adapter) SetOrigin(string) {
	panic("interface Resource153 does not implement SetOrigin")
}

func (r UnifiedResource153Adapter) GetLabel(key string) (value string, ok bool) {
	m := r.Inner.GetMetadata()
	if m == nil {
		return "", false
	}
	value, ok = m.Labels[key]
	return
}

func (r UnifiedResource153Adapter) GetAllLabels() map[string]string {
	m := r.Inner.GetMetadata()
	if m == nil {
		return nil
	}
	return m.Labels
}

func (r UnifiedResource153Adapter) GetStaticLabels() map[string]string {
	return r.GetAllLabels()
}

func (r UnifiedResource153Adapter) SetStaticLabels(map[string]string) {
	panic("interface Resource153 does not implement SetStaticLabels")
}

func (r UnifiedResource153Adapter) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName())
	return types.MatchSearch(fieldVals, searchValues, nil)
}

func (r UnifiedResource153Adapter) CloneResource() types.ResourceWithLabels {
	return UnifiedResource153Adapter{Inner: r.Inner.CloneResource().(UnifiedResource153)}
}
