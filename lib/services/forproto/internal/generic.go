// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package internal

import (
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protowire"
)

// Driver is the type that contains the functions required by [Resource] to
// manipulate an underlying resource.
type Driver[T any, E any] interface {
	// Marshal returns the protobuf wire format encoding of a resource.
	Marshal(*T) (string, error)
	// Unmarshal returns a new resource given the protobuf wire format encoding
	// of it.
	Unmarshal(string) (*T, error)

	// GetExtra extracts the extra data associated with a resource.
	GetExtra(*T) E
}

// DriverFieldOne is a [Driver] that also flags the type as having an associated
// List operation for the resource that uses a repeated protobuf field with
// message number 1.
type DriverFieldOne[T any, E any] interface {
	Driver[T, E]

	ListUsesFieldOne()
}

// Resource is a resource that can either be an object in memory or can be in
// marshaled protobuf format. The ownership of the resource can be assumed to
// belong to the Resource.
type Resource[T any, E any, D Driver[T, E]] struct {
	// Actual, if non-nil, is the resource in its unmarshaled form.
	Actual *T
	// Wire, if Actual is nil, is the resource in its binary protobuf form.
	Wire string

	// Extra is additional data that should be accessible even if the resource
	// is in its marshaled form.
	Extra E
}

// SetActual replaces the Resource with an unmarshaled Resource owning the given
// actual resource.
func (r *Resource[T, E, D]) SetActual(actual *T) {
	*r = Resource[T, E, D]{
		Actual: actual,
		Wire:   "",
		Extra:  (*new(D)).GetExtra(actual),
	}
}

// SetWire replaces the Resource with a marshaled Resource, given the existing
// protobuf wire encoding. The actual resource is assumed to be the source of
// the wire format, and it's only inspected to derive the extra data for the
// resource, not stored.
func (r *Resource[T, E, D]) SetWire(actual *T, wire string) {
	*r = Resource[T, E, D]{
		Actual: nil,
		Wire:   wire,
		Extra:  (*new(D)).GetExtra(actual),
	}
}

// ToWire marshals and discards the actual resource in the Resource, storing its
// encoded form. If the resource is already encoded, it does nothing.
func (r *Resource[T, E, D]) ToWire() error {
	if r.Actual == nil {
		return nil
	}
	wire, err := (*new(D)).Marshal(r.Actual)
	if err != nil {
		return trace.Wrap(err)
	}
	r.Wire = wire
	r.Actual = nil
	return nil
}

// ToActual unmarshals and discards the wire format in the Resource, storing the
// resulting resource in it. If the resource is already unmarshaled, it does
// nothing.
func (r *Resource[T, E, D]) ToActual() error {
	if r.Actual != nil {
		return nil
	}
	actual, err := (*new(D)).Unmarshal(r.Wire)
	if err != nil {
		return trace.Wrap(err)
	}
	r.Actual = actual
	r.Wire = ""
	return nil
}

// wrapper is what we expect the actual forproto types to be, an embedded
// Resource with a fixed resource type, extra data and driver.
type wrapper[T any, E any, D Driver[T, E]] = interface {
	~struct{ Resource[T, E, D] }
}

// ResourcesToProtoFieldOne converts a list of (wrapped) Resources into a slice
// of actual resources or into a manually marshaled protobuf message storing
// resources as a repeated field with message number 1. The list of Resources
// will be manually marshaled if any of the resources is already marshaled.
func ResourcesToProtoFieldOne[R wrapper[T, E, D], T any, E any, D DriverFieldOne[T, E]](resources []R) ([]*T, []byte, error) {
	for _, resource := range resources {
		var unwrapped struct{ Resource[T, E, D] } = resource
		if unwrapped.Actual == nil {
			wire, err := resourcesToProtoFieldOneMarshaled(resources)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			return nil, wire, nil
		}
	}

	actuals := make([]*T, 0, len(resources))
	for _, resource := range resources {
		var unwrapped struct{ Resource[T, E, D] } = resource
		if err := unwrapped.ToActual(); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		actuals = append(actuals, unwrapped.Actual)
	}
	return actuals, nil, nil
}

func resourcesToProtoFieldOneMarshaled[R wrapper[T, E, D], T any, E any, D DriverFieldOne[T, E]](resources []R) ([]byte, error) {
	const fieldNumber = protowire.Number(1)
	const fieldNumberTagSize = 1
	const fieldNumberTypeBytesTag = uint64(fieldNumber<<3) | uint64(protowire.BytesType&7)
	const fieldNumberTypeBytesTagVarint = byte(fieldNumberTypeBytesTag)

	var totalLen = len(resources) * fieldNumberTagSize
	for i, resource := range resources {
		var unwrapped struct{ Resource[T, E, D] } = resource
		if err := unwrapped.ToWire(); err != nil {
			return nil, trace.Wrap(err)
		}
		resources[i] = unwrapped
		totalLen += protowire.SizeBytes(len(unwrapped.Wire))
	}

	buf := make([]byte, 0, totalLen)
	for _, resource := range resources {
		var unwrapped struct{ Resource[T, E, D] } = resource
		buf = append(buf, fieldNumberTypeBytesTagVarint)
		buf = protowire.AppendString(buf, unwrapped.Wire)
	}

	return buf, nil
}

// ResourcesFromActual converts a list of actual resources into a list of
// Resources in unmarshaled form.
func ResourcesFromActual[R wrapper[T, E, D], T any, E any, D Driver[T, E]](actuals []*T) []R {
	resources := make([]R, 0, len(actuals))
	for _, actual := range actuals {
		var w struct{ Resource[T, E, D] }
		w.SetActual(actual)
		resources = append(resources, w)
	}
	return resources
}
