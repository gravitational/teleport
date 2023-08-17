/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package local

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local/generic"
)

const (
	accessListPrefix = "access_list"
	// We'll use a small page size here to minimize the potential for access lists with large member lists
	// from hitting the gRPC receive limit.
	accessListMaxPageSize = 4
)

// AccessListService manages Access List resources in the Backend.
type AccessListService struct {
	log   logrus.FieldLogger
	clock clockwork.Clock
	svc   *generic.Service[*accesslist.AccessList]
}

// NewAccessListService creates a new AccessListService.
func NewAccessListService(backend backend.Backend, clock clockwork.Clock) (*AccessListService, error) {
	svc, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessList]{
		Backend:       backend,
		PageLimit:     accessListMaxPageSize,
		ResourceKind:  types.KindAccessList,
		BackendPrefix: accessListPrefix,
		MarshalFunc:   services.MarshalAccessList,
		UnmarshalFunc: services.UnmarshalAccessList,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		log:   logrus.WithFields(logrus.Fields{trace.Component: "access-list:local-service"}),
		clock: clock,
		svc:   svc,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (a *AccessListService) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	accessLists, err := a.svc.GetResources(ctx)
	return accessLists, trace.Wrap(err)
}

// ListAccessLists returns a paginated list of access lists.
func (a *AccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	return a.svc.ListResources(ctx, pageSize, nextToken)
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	accessList, err := a.svc.GetResource(ctx, name)
	return accessList, trace.Wrap(err)
}

// UpsertAccessList creates or updates an access list resource.
func (a *AccessListService) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	if err := trace.Wrap(a.svc.UpsertResource(ctx, accessList)); err != nil {
		return nil, trace.Wrap(err)
	}
	return accessList, nil
}

// DeleteAccessList removes the specified access list resource.
func (a *AccessListService) DeleteAccessList(ctx context.Context, name string) error {
	return trace.Wrap(a.svc.DeleteResource(ctx, name))
}

// DeleteAllAccessLists removes all access lists.
func (a *AccessListService) DeleteAllAccessLists(ctx context.Context) error {
	return trace.Wrap(a.svc.DeleteAllResources(ctx))
}
