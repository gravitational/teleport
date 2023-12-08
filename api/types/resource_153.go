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

// Resource153ToLegacy transforms an RFD 153 style resource into a legacy
// [Resource] type.
//
// Note that CheckAndSetDefaults is a noop for the returned resource and
// SetSubKind is not implemented and panics on use.
func Resource153ToLegacy(r Resource153) Resource {
	return &resource153ToLegacyAdapter{inner: r}
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

func (r *resource153ToLegacyAdapter) CheckAndSetDefaults() error {
	// Validation is not a distributed responsibility!
	// Write your own validation, against the concrete type, in the storage layer.
	// https://github.com/gravitational/teleport/pull/34103/files#diff-49c80914f68671852ea118fbd508af507b6b59b196b48a404c658e3eb9f1bf78R309
	// TODO(rossstimothy): Update link above when RFD 153 lands.
	return nil
}

func (r *resource153ToLegacyAdapter) Expiry() time.Time {
	return r.inner.GetMetadata().Expires.AsTime()
}

func (r *resource153ToLegacyAdapter) GetKind() string {
	return r.inner.GetKind()
}

func (r *resource153ToLegacyAdapter) GetMetadata() Metadata {
	md := r.inner.GetMetadata()
	expires := md.Expires.AsTime()
	return Metadata{
		Name:        md.Name,
		Namespace:   md.Namespace,
		Description: md.Description,
		Labels:      md.Labels,
		Expires:     &expires,
		ID:          md.Id,
		Revision:    md.Revision,
	}
}

func (r *resource153ToLegacyAdapter) GetName() string {
	return r.inner.GetMetadata().Name
}

func (r *resource153ToLegacyAdapter) GetResourceID() int64 {
	//nolint:deprecated // We need to refer to Id to provide GetResourceID.
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
	//nolint:deprecated // We need to refer to Id to provide SetResourceID.
	r.inner.GetMetadata().Id = id
}

func (r *resource153ToLegacyAdapter) SetRevision(rev string) {
	r.inner.GetMetadata().Revision = rev
}

func (r *resource153ToLegacyAdapter) SetSubKind(subKind string) {
	panic("interface Resource153 does not implement SetSubKind")
}
