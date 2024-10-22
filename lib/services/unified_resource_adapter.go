package services

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

type UnifiedResource153[T interface{ CloneResource() T }] interface {
	types.Resource153
	CloneResource() T
}

type Resource153Adapter[T UnifiedResource153[T]] struct {
	Inner T
}

func (r Resource153Adapter[T]) Unwrap() T {
	return r.Inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r Resource153Adapter[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Inner)
}

func (r Resource153Adapter[T]) Expiry() time.Time {
	expires := r.Inner.GetMetadata().Expires
	// return zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	if expires == nil {
		return time.Time{}
	}

	return expires.AsTime()
}

func (r Resource153Adapter[T]) GetKind() string {
	return r.Inner.GetKind()
}

func (r Resource153Adapter[T]) GetMetadata() types.Metadata {
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

func (r Resource153Adapter[T]) GetName() string {
	return r.Inner.GetMetadata().Name
}

func (r Resource153Adapter[T]) GetRevision() string {
	return r.Inner.GetMetadata().Revision
}

func (r Resource153Adapter[T]) GetSubKind() string {
	return r.Inner.GetSubKind()
}

func (r Resource153Adapter[T]) GetVersion() string {
	return r.Inner.GetVersion()
}

func (r Resource153Adapter[T]) SetExpiry(t time.Time) {
	r.Inner.GetMetadata().Expires = timestamppb.New(t)
}

func (r Resource153Adapter[T]) SetName(name string) {
	r.Inner.GetMetadata().Name = name
}

func (r Resource153Adapter[T]) SetRevision(rev string) {
	r.Inner.GetMetadata().Revision = rev
}

func (r Resource153Adapter[T]) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}

func (r Resource153Adapter[T]) Origin() string {
	m := r.Inner.GetMetadata()
	if m == nil {
		return ""
	}
	return m.Labels[types.OriginLabel]
}

func (r Resource153Adapter[T]) SetOrigin(string) {
	panic("interface Resource153 does not implement SetOrigin")
}

func (r Resource153Adapter[T]) GetLabel(key string) (value string, ok bool) {
	m := r.Inner.GetMetadata()
	if m == nil {
		return "", false
	}
	value, ok = m.Labels[key]
	return
}

func (r Resource153Adapter[T]) GetAllLabels() map[string]string {
	m := r.Inner.GetMetadata()
	if m == nil {
		return nil
	}
	return m.Labels
}

func (r Resource153Adapter[T]) GetStaticLabels() map[string]string {
	return r.GetAllLabels()
}

func (r Resource153Adapter[T]) SetStaticLabels(map[string]string) {
	panic("interface Resource153 does not implement SetStaticLabels")
}

func (r Resource153Adapter[T]) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName())
	return types.MatchSearch(fieldVals, searchValues, nil)
}

func (r Resource153Adapter[T]) CloneResource() types.ResourceWithLabels {
	return Resource153Adapter[T]{Inner: r.Inner.CloneResource()}
}
