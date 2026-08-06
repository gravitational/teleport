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

package tfdriver

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/services/resourceregistry"
)

// RegistryIdentifier converts a Terraform identifier into a registry
// identifier.
type RegistryIdentifier[I Identifier] func(I) resourceregistry.NameID

// NameIdentifierAsRegistryID adapts Terraform's ordinary name identifier.
func NameIdentifierAsRegistryID(id NameIdentifier) resourceregistry.NameID {
	return resourceregistry.NameID(id.Name)
}

// NewRegistryDataSourceClient adapts a registry client to tfdriver's data
// source client shape. tfdriver stores proto resources as values, while the
// resource registry stores them as pointers, so this adapter bridges that
// representation detail.
func NewRegistryDataSourceClient[T any, I Identifier](
	providerClient any,
	spec resourceregistry.Spec[*T, resourceregistry.NameID],
	identifier RegistryIdentifier[I],
) DataSourceClient[T, I] {
	reader, err := spec.ReaderFor(providerClient)
	return registryDataSourceClient[T, I]{
		reader:     reader,
		readerErr:  err,
		identifier: identifier,
	}
}

// NewRegistryResourceClient adapts a registry client to tfdriver's resource
// client shape.
func NewRegistryResourceClient[T any, I Identifier](
	providerClient any,
	spec resourceregistry.Spec[*T, resourceregistry.NameID],
	identifier RegistryIdentifier[I],
) ResourceClient[T, I] {
	client, err := spec.Client(providerClient)
	return registryClient[T, I]{
		client:     client,
		clientErr:  err,
		identifier: identifier,
	}
}

type registryDataSourceClient[T any, I Identifier] struct {
	reader     resourceregistry.Reader[*T, resourceregistry.NameID]
	readerErr  error
	identifier RegistryIdentifier[I]
}

func (r registryDataSourceClient[T, I]) check() error {
	if r.readerErr != nil {
		return trace.Wrap(r.readerErr)
	}
	if r.identifier == nil {
		return trace.BadParameter("missing registry identifier adapter")
	}
	return nil
}

func (r registryDataSourceClient[T, I]) Get(ctx context.Context, id I) (*T, error) {
	if err := r.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	resource, err := r.reader.Get(ctx, r.identifier(id))
	return resource, trace.Wrap(err)
}

type registryClient[T any, I Identifier] struct {
	client     resourceregistry.Client[*T, resourceregistry.NameID]
	clientErr  error
	identifier RegistryIdentifier[I]
}

func (r registryClient[T, I]) check() error {
	if r.clientErr != nil {
		return trace.Wrap(r.clientErr)
	}
	if r.identifier == nil {
		return trace.BadParameter("missing registry identifier adapter")
	}
	return nil
}

func (r registryClient[T, I]) Get(ctx context.Context, id I) (*T, error) {
	if err := r.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	resource, err := r.client.Get(ctx, r.identifier(id))
	return resource, trace.Wrap(err)
}

func (r registryClient[T, I]) Create(ctx context.Context, resource *T) error {
	if err := r.check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := r.client.Create(ctx, resource)
	return trace.Wrap(err)
}

func (r registryClient[T, I]) Upsert(ctx context.Context, resource *T) error {
	if err := r.check(); err != nil {
		return trace.Wrap(err)
	}
	_, err := r.client.Update(ctx, resource)
	return trace.Wrap(err)
}

func (r registryClient[T, I]) Delete(ctx context.Context, id I) error {
	if err := r.check(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.client.Delete(ctx, r.identifier(id)))
}
