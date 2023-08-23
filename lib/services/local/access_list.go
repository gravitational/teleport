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
	accessListPrefix      = "access_list"
	accessListMaxPageSize = 100

	accessListMemberPrefix      = "access_list_member"
	accessListMemberMaxPageSize = 200
)

// AccessListService manages Access List resources in the Backend.
type AccessListService struct {
	log           logrus.FieldLogger
	clock         clockwork.Clock
	service       *generic.Service[*accesslist.AccessList]
	memberService *generic.Service[*accesslist.AccessListMember]
}

// NewAccessListService creates a new AccessListService.
func NewAccessListService(backend backend.Backend, clock clockwork.Clock) (*AccessListService, error) {
	service, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessList]{
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

	memberService, err := generic.NewService(&generic.ServiceConfig[*accesslist.AccessListMember]{
		Backend:       backend,
		PageLimit:     accessListMemberMaxPageSize,
		ResourceKind:  types.KindAccessListMember,
		BackendPrefix: accessListMemberPrefix,
		MarshalFunc:   services.MarshalAccessListMember,
		UnmarshalFunc: services.UnmarshalAccessListMember,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &AccessListService{
		log:           logrus.WithFields(logrus.Fields{trace.Component: "access-list:local-service"}),
		clock:         clock,
		service:       service,
		memberService: memberService,
	}, nil
}

// GetAccessLists returns a list of all access lists.
func (a *AccessListService) GetAccessLists(ctx context.Context) ([]*accesslist.AccessList, error) {
	accessLists, err := a.service.GetResources(ctx)
	return accessLists, trace.Wrap(err)
}

// ListAccessLists returns a paginated list of access lists.
func (a *AccessListService) ListAccessLists(ctx context.Context, pageSize int, nextToken string) ([]*accesslist.AccessList, string, error) {
	return a.service.ListResources(ctx, pageSize, nextToken)
}

// GetAccessList returns the specified access list resource.
func (a *AccessListService) GetAccessList(ctx context.Context, name string) (*accesslist.AccessList, error) {
	accessList, err := a.service.GetResource(ctx, name)
	return accessList, trace.Wrap(err)
}

// UpsertAccessList creates or updates an access list resource.
func (a *AccessListService) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	if err := a.service.UpsertResource(ctx, accessList); err != nil {
		return nil, trace.Wrap(err)
	}
	return accessList, nil
}

// DeleteAccessList removes the specified access list resource.
func (a *AccessListService) DeleteAccessList(ctx context.Context, name string) error {
	// Delete all associated members.
	err := a.DeleteAllAccessListMembersForAccessList(ctx, name)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.service.DeleteResource(ctx, name))
}

// DeleteAllAccessLists removes all access lists.
func (a *AccessListService) DeleteAllAccessLists(ctx context.Context) error {
	// Delete all members for all access lists.
	err := a.DeleteAllAccessListMembers(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.service.DeleteAllResources(ctx))
}

// ListAccessListMembers returns a paginated list of all access list members.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessList string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	// Make sure the access list is present.
	_, err := a.service.GetResource(ctx, accessList)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return a.memberService.WithPrefix(accessList).ListResources(ctx, pageSize, nextToken)
}

// GetAccessListMember returns the specified access list member resource.
func (a *AccessListService) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	// Make sure the access list is present.
	_, err := a.service.GetResource(ctx, accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	member, err := a.memberService.WithPrefix(accessList).GetResource(ctx, memberName)
	return member, trace.Wrap(err)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (a *AccessListService) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	// Make sure the access list is present.
	_, err := a.service.GetResource(ctx, member.Spec.AccessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := trace.Wrap(a.memberService.WithPrefix(member.Spec.AccessList).UpsertResource(ctx, member)); err != nil {
		return nil, trace.Wrap(err)
	}
	return member, nil
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (a *AccessListService) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	// Make sure the access list is present.
	_, err := a.service.GetResource(ctx, accessList)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.memberService.WithPrefix(accessList).DeleteResource(ctx, memberName))
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list.
func (a *AccessListService) DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error {
	// Make sure the access list is present.
	_, err := a.service.GetResource(ctx, accessList)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.memberService.WithPrefix(accessList).DeleteAllResources(ctx))
}

// DeleteAllAccessListMembers hard deletes all access list members.
func (a *AccessListService) DeleteAllAccessListMembers(ctx context.Context) error {
	return trace.Wrap(a.memberService.DeleteAllResources(ctx))
}
