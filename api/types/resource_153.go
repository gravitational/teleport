// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types

import (
	"encoding/json"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/utils"
)

// ResourceMetadata is the smallest interface that defines a Teleport resource.
type ResourceMetadata interface {
	// GetMetadata returns the generic resource metadata.
	GetMetadata() *headerv1.Metadata
}

// Resource153 is a resource that follows RFD 153.
//
// It exists as a weak guideline for fields that resource protos must provide
// and as a way to adapt "new" resources to the legacy [Resource] interface.
//
// Strongly prefer using actual types, like *myprotov1.Foo, instead of this
// interface. If you do need to represent resources in a generic manner,
// consider declaring a smaller interface with only what you need.
//
// Embedding or further extending this interface is highly discouraged.
type Resource153 interface {
	// GetKind returns the resource kind.
	//
	// Kind is usually hard-coded for each underlying type.
	GetKind() string

	// GetSubKind returns the resource sub-kind, if any.
	GetSubKind() string

	// GetVersion returns the resource API version.
	//
	// See [headerv1.Metadata.Revision] for an identifier of the resource over
	// time.
	GetVersion() string

	// GetMetadata returns the generic resource metadata.
	GetMetadata() *headerv1.Metadata
}

// LegacyToResource153 converts a legacy [Resource] into a [Resource153].
//
// Useful to handle old and new resources uniformly. If you can, consider
// further "downgrading" the Resource153 interface into the smallest subset that
// works for you (for example, [ResourceMetadata]).
func LegacyToResource153(r Resource) Resource153 {
	return &legacyToResource153Adapter{inner: r}
}

type legacyToResource153Adapter struct {
	inner Resource
}

// UnwrapT is an escape hatch for Resource instances that are piped down into the
// codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *legacyToResource153Adapter) UnwrapT() Resource {
	return r.inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r *legacyToResource153Adapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.inner)
}

func (r *legacyToResource153Adapter) GetKind() string {
	return r.inner.GetKind()
}

// LegacyTo153Metadata converts a legacy [Metadata] object an RFD153-style
// [headerv1.Metadata] block
func LegacyTo153Metadata(md Metadata) *headerv1.Metadata {
	var expires *timestamppb.Timestamp
	if md.Expires != nil {
		expires = timestamppb.New(*md.Expires)
	}

	return &headerv1.Metadata{
		Name:        md.Name,
		Namespace:   md.Namespace,
		Description: md.Description,
		Labels:      md.Labels,
		Expires:     expires,
		Revision:    md.Revision,
	}
}

func (r *legacyToResource153Adapter) GetMetadata() *headerv1.Metadata {
	return LegacyTo153Metadata(r.inner.GetMetadata())
}

func (r *legacyToResource153Adapter) GetSubKind() string {
	return r.inner.GetSubKind()
}

func (r *legacyToResource153Adapter) GetVersion() string {
	return r.inner.GetVersion()
}

// Resource153ToLegacy transforms an RFD 153 style resource into a legacy
// [Resource] type. Implements [ResourceWithLabels] and CloneResource (where the)
// wrapped resource supports cloning).
//
// Resources153 implemented by proto-generated structs should use ProtoResource153ToLegacy
// instead as it will ensure the protobuf message is properly marshaled to JSON
// with protojson.
//
// Note that CheckAndSetDefaults is a noop for the returned resource and
// SetSubKind is not implemented and panics on use.
func Resource153ToLegacy[T Resource153](r T) Resource {
	return &resource153ToLegacyAdapter[T]{inner: r}
}

// Resource153UnwrapperT returns a [T] from a wrapped RFD
// 153 style resource.
type Resource153UnwrapperT[T Resource153] interface{ UnwrapT() T }

// resource153ToLegacyAdapter wraps a new-style resource in a type implementing
// the legacy resource interfaces
type resource153ToLegacyAdapter[T Resource153] struct {
	inner T
}

// UnwrapT is an escape hatch for Resource153 instances that are piped down into
// the codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *resource153ToLegacyAdapter[T]) UnwrapT() T {
	return r.inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r *resource153ToLegacyAdapter[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.inner)
}

func (r *resource153ToLegacyAdapter[T]) Expiry() time.Time {
	expires := r.inner.GetMetadata().Expires
	// return zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	if expires == nil {
		return time.Time{}
	}

	return expires.AsTime()
}

func (r *resource153ToLegacyAdapter[T]) GetKind() string {
	return r.inner.GetKind()
}

// Metadata153ToLegacy converts RFD153-style resource metadata to legacy
// metadata.
func Metadata153ToLegacy(md *headerv1.Metadata) Metadata {
	// use zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	expires := md.Expires.AsTime()
	if md.Expires == nil {
		expires = time.Time{}
	}

	return Metadata{
		Name:        md.Name,
		Namespace:   md.Namespace,
		Description: md.Description,
		Labels:      md.Labels,
		Expires:     &expires,
		Revision:    md.Revision,
	}
}

func (r *resource153ToLegacyAdapter[T]) GetMetadata() Metadata {
	return Metadata153ToLegacy(r.inner.GetMetadata())
}

func (r *resource153ToLegacyAdapter[T]) GetName() string {
	return r.inner.GetMetadata().Name
}

func (r *resource153ToLegacyAdapter[T]) GetRevision() string {
	return r.inner.GetMetadata().Revision
}

func (r *resource153ToLegacyAdapter[T]) GetSubKind() string {
	return r.inner.GetSubKind()
}

func (r *resource153ToLegacyAdapter[T]) GetVersion() string {
	return r.inner.GetVersion()
}

func (r *resource153ToLegacyAdapter[T]) SetExpiry(t time.Time) {
	r.inner.GetMetadata().Expires = timestamppb.New(t)
}

func (r *resource153ToLegacyAdapter[T]) SetName(name string) {
	r.inner.GetMetadata().Name = name
}

func (r *resource153ToLegacyAdapter[T]) SetRevision(rev string) {
	r.inner.GetMetadata().Revision = rev
}

func (r *resource153ToLegacyAdapter[T]) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}

// Resource153ToResourceWithLabels wraps a [Resource153]-style resource in
// the legacy [Resource] and [ResourceWithLabels] interfaces.
//
// The same caveats that apply to [Resource153ToLegacy] apply.
func Resource153ToResourceWithLabels[T Resource153](r T) ResourceWithLabels {
	return &resource153ToResourceWithLabelsAdapter[T]{
		resource153ToLegacyAdapter[T]{
			inner: r,
		},
	}
}

// resource153ToResourceWithLabelsAdapter wraps a new-style resource in a
// type implementing the legacy resource interfaces
type resource153ToResourceWithLabelsAdapter[T Resource153] struct {
	resource153ToLegacyAdapter[T]
}

// UnwrapT is an escape hatch for Resource153 instances that are piped down into
// the codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *resource153ToResourceWithLabelsAdapter[T]) UnwrapT() T {
	return r.inner
}

// Origin implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) Origin() string {
	m := r.inner.GetMetadata()
	if m == nil {
		return ""
	}
	return m.Labels[OriginLabel]
}

// SetOrigin implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) SetOrigin(origin string) {
	m := r.inner.GetMetadata()
	if m == nil {
		return
	}
	m.Labels[OriginLabel] = origin
}

// GetLabel implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) GetLabel(key string) (value string, ok bool) {
	m := r.inner.GetMetadata()
	if m == nil {
		return "", false
	}
	value, ok = m.Labels[key]
	return
}

// GetAllLabels implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) GetAllLabels() map[string]string {
	m := r.inner.GetMetadata()
	if m == nil {
		return nil
	}
	return m.Labels
}

// GetStaticLabels implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) GetStaticLabels() map[string]string {
	return r.GetAllLabels()
}

// SetStaticLabels implements ResourceWithLabels for the adapter.
func (r *resource153ToResourceWithLabelsAdapter[T]) SetStaticLabels(labels map[string]string) {
	m := r.inner.GetMetadata()
	if m == nil {
		return
	}
	m.Labels = labels
}

// MatchSearch implements ResourceWithLabels for the adapter. If the underlying
// type exposes a MatchSearch method, this method will defer to that, otherwise
// it will match against the resource label values and name.
func (r *resource153ToResourceWithLabelsAdapter[T]) MatchSearch(searchValues []string) bool {
	if matcher, ok := any(r.inner).(interface{ MatchSearch([]string) bool }); ok {
		return matcher.MatchSearch(searchValues)
	}
	fieldVals := append(utils.MapToStrings(r.GetAllLabels()), r.GetName())
	return MatchSearch(fieldVals, searchValues, nil)
}

// ProtoResource153 is a Resource153 implemented by a protobuf-generated struct.
type ProtoResource153 interface {
	Resource153
	proto.Message
}

type protoResource153ToLegacyAdapter[T ProtoResource153] struct {
	inner T
	resource153ToLegacyAdapter[T]
}

// UnwrapT is an escape hatch for Resource153 instances that are piped down into
// the codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *protoResource153ToLegacyAdapter[T]) UnwrapT() T {
	return r.inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r *protoResource153ToLegacyAdapter[T]) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{
		UseProtoNames: true,
	}.Marshal(r.inner)
}

// ProtoResource153ToLegacy transforms an RFD 153 style resource implemented by
// a proto-generated struct into a legacy [Resource] type. Implements
// [ResourceWithLabels] and CloneResource (where the wrapped resource supports
// cloning).
//
// Note that CheckAndSetDefaults is a noop for the returned resource and
// SetSubKind is not implemented and panics on use.
func ProtoResource153ToLegacy[T ProtoResource153](r T) Resource {
	return &protoResource153ToLegacyAdapter[T]{
		r,
		resource153ToLegacyAdapter[T]{r},
	}
}
