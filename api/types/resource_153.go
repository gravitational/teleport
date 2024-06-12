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

	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
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

// Unwrap is an escape hatch for Resource instances that are piped down into the
// codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *legacyToResource153Adapter) Unwrap() Resource {
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

func (r *legacyToResource153Adapter) GetMetadata() *headerv1.Metadata {
	md := r.inner.GetMetadata()

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
		Id:          md.ID,
		Revision:    md.Revision,
	}
}

func (r *legacyToResource153Adapter) GetSubKind() string {
	return r.inner.GetSubKind()
}

func (r *legacyToResource153Adapter) GetVersion() string {
	return r.inner.GetVersion()
}

// Resource153ToLegacy transforms an RFD 153 style resource into a legacy
// [Resource] type.
//
// Note that CheckAndSetDefaults is a noop for the returned resource and
// SetSubKind is not implemented and panics on use.
func Resource153ToLegacy(r Resource153) Resource {
	return &resource153ToLegacyAdapter{inner: r}
}

// Resource153Unwrapper returns a legacy [Resource] type from a wrapped RFD
// 153 style resource
type Resource153Unwrapper interface {
	Unwrap() Resource153
}

type resource153ToLegacyAdapter struct {
	inner Resource153
}

// Unwrap is an escape hatch for Resource153 instances that are piped down into
// the codebase as a legacy Resource.
//
// Ideally you shouldn't depend on this.
func (r *resource153ToLegacyAdapter) Unwrap() Resource153 {
	return r.inner
}

// MarshalJSON adds support for marshaling the wrapped resource (instead of
// marshaling the adapter itself).
func (r *resource153ToLegacyAdapter) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.inner)
}

func (r *resource153ToLegacyAdapter) Expiry() time.Time {
	expires := r.inner.GetMetadata().Expires
	// return zero time.time{} for zero *timestamppb.Timestamp, instead of 01/01/1970.
	if expires == nil {
		return time.Time{}
	}

	return expires.AsTime()
}

func (r *resource153ToLegacyAdapter) GetKind() string {
	return r.inner.GetKind()
}

func (r *resource153ToLegacyAdapter) GetMetadata() Metadata {
	md := r.inner.GetMetadata()

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
		//nolint:staticcheck // SA1019. We need to refer to Id to create the Metadata.
		ID:       md.Id,
		Revision: md.Revision,
	}
}

func (r *resource153ToLegacyAdapter) GetName() string {
	return r.inner.GetMetadata().Name
}

func (r *resource153ToLegacyAdapter) GetResourceID() int64 {
	//nolint:staticcheck // SA1019. We need to refer to Id to provide GetResourceID.
	return r.inner.GetMetadata().Id
}

func (r *resource153ToLegacyAdapter) GetRevision() string {
	return r.inner.GetMetadata().Revision
}

func (r *resource153ToLegacyAdapter) GetSubKind() string {
	return r.inner.GetSubKind()
}

func (r *resource153ToLegacyAdapter) GetVersion() string {
	return r.inner.GetVersion()
}

func (r *resource153ToLegacyAdapter) SetExpiry(t time.Time) {
	r.inner.GetMetadata().Expires = timestamppb.New(t)
}

func (r *resource153ToLegacyAdapter) SetName(name string) {
	r.inner.GetMetadata().Name = name
}

func (r *resource153ToLegacyAdapter) SetResourceID(id int64) {
	//nolint:staticcheck // SA1019. We need to refer to Id to provide SetResourceID.
	r.inner.GetMetadata().Id = id
}

func (r *resource153ToLegacyAdapter) SetRevision(rev string) {
	r.inner.GetMetadata().Revision = rev
}

func (r *resource153ToLegacyAdapter) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}
