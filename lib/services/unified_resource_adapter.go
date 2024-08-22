package services

import (
	"encoding/json"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type unifiedResource153[T interface{ CloneResource() T }] interface {
	types.Resource153
	CloneResource() T
}

type resource153Adapter[T unifiedResource153[T]] struct {
	inner T
}

func (r resource153Adapter[T]) Unwrap() T {
	return r.inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r resource153Adapter[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.inner)
}

func (r resource153Adapter[T]) Expiry() time.Time {
	expires := r.inner.GetMetadata().Expires
	// return zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	if expires == nil {
		return time.Time{}
	}

	return expires.AsTime()
}

func (r resource153Adapter[T]) GetKind() string {
	return r.inner.GetKind()
}

func (r resource153Adapter[T]) GetMetadata() types.Metadata {
	md := r.inner.GetMetadata()

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

func (r resource153Adapter[T]) GetName() string {
	return r.inner.GetMetadata().Name
}

func (r resource153Adapter[T]) GetRevision() string {
	return r.inner.GetMetadata().Revision
}

func (r resource153Adapter[T]) GetSubKind() string {
	return r.inner.GetSubKind()
}

func (r resource153Adapter[T]) GetVersion() string {
	return r.inner.GetVersion()
}

func (r resource153Adapter[T]) SetExpiry(t time.Time) {
	r.inner.GetMetadata().Expires = timestamppb.New(t)
}

func (r resource153Adapter[T]) SetName(name string) {
	r.inner.GetMetadata().Name = name
}

func (r resource153Adapter[T]) SetRevision(rev string) {
	r.inner.GetMetadata().Revision = rev
}

func (r resource153Adapter[T]) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}

func (r resource153Adapter[T]) Origin() string {
	m := r.inner.GetMetadata()
	if m == nil {
		return ""
	}
	return m.Labels[types.OriginLabel]
}

func (r resource153Adapter[T]) SetOrigin(string) {
	panic("interface Resource153 does not implement SetOrigin")
}

func (r resource153Adapter[T]) GetLabel(key string) (value string, ok bool) {
	m := r.inner.GetMetadata()
	if m == nil {
		return "", false
	}
	value, ok = m.Labels[key]
	return
}

func (r resource153Adapter[T]) GetAllLabels() map[string]string {
	m := r.inner.GetMetadata()
	if m == nil {
		return nil
	}
	return m.Labels
}

func (r resource153Adapter[T]) GetStaticLabels() map[string]string {
	return r.GetAllLabels()
}

func (r resource153Adapter[T]) SetStaticLabels(map[string]string) {
	panic("interface Resource153 does not implement SetStaticLabels")
}

func (r resource153Adapter[T]) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName())
	return types.MatchSearch(fieldVals, searchValues, nil)
}

func (r resource153Adapter[T]) CloneResource() types.ResourceWithLabels {
	return resource153Adapter[T]{inner: r.inner.CloneResource()}
}
