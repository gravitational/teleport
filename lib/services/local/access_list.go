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
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
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

	// This lock is necessary to prevent a race condition between access lists and members and to ensure
	// consistency of the one-to-many relationship between them.
	accessListLockTTL = 5 * time.Second
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
	var accessList *accesslist.AccessList
	err := a.service.RunWhileLocked(ctx, lockName(name), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		var err error
		accessList, err = a.service.GetResource(ctx, name)
		return trace.Wrap(err)
	})
	return accessList, trace.Wrap(err)
}

// UpsertAccessList creates or updates an access list resource.
func (a *AccessListService) UpsertAccessList(ctx context.Context, accessList *accesslist.AccessList) (*accesslist.AccessList, error) {
	err := a.service.RunWhileLocked(ctx, lockName(accessList.GetName()), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		return trace.Wrap(a.service.UpsertResource(ctx, accessList))
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return accessList, nil
}

// DeleteAccessList removes the specified access list resource.
func (a *AccessListService) DeleteAccessList(ctx context.Context, name string) error {
	err := a.service.RunWhileLocked(ctx, lockName(name), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		// Delete all associated members.
		err := a.memberService.WithPrefix(name).DeleteAllResources(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(a.service.DeleteResource(ctx, name))
	})

	return trace.Wrap(err)
}

// DeleteAllAccessLists removes all access lists.
func (a *AccessListService) DeleteAllAccessLists(ctx context.Context) error {
	// Locks are not used here as these operations are more likely to be used by the cache.
	// Delete all members for all access lists.
	err := a.memberService.DeleteAllResources(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(a.service.DeleteAllResources(ctx))
}

// ListAccessListMembers returns a paginated list of all access list members.
func (a *AccessListService) ListAccessListMembers(ctx context.Context, accessList string, pageSize int, nextToken string) ([]*accesslist.AccessListMember, string, error) {
	var members []*accesslist.AccessListMember
	err := a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		members, nextToken, err = a.memberService.WithPrefix(accessList).ListResources(ctx, pageSize, nextToken)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return members, nextToken, nil
}

// GetAccessListMember returns the specified access list member resource.
func (a *AccessListService) GetAccessListMember(ctx context.Context, accessList string, memberName string) (*accesslist.AccessListMember, error) {
	var member *accesslist.AccessListMember
	err := a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		member, err = a.memberService.WithPrefix(accessList).GetResource(ctx, memberName)
		return trace.Wrap(err)
	})
	return member, trace.Wrap(err)
}

// UpsertAccessListMember creates or updates an access list member resource.
func (a *AccessListService) UpsertAccessListMember(ctx context.Context, member *accesslist.AccessListMember) (*accesslist.AccessListMember, error) {
	err := a.service.RunWhileLocked(ctx, lockName(member.Spec.AccessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, member.Spec.AccessList)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(a.memberService.WithPrefix(member.Spec.AccessList).UpsertResource(ctx, member))
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return member, nil
}

// DeleteAccessListMember hard deletes the specified access list member resource.
func (a *AccessListService) DeleteAccessListMember(ctx context.Context, accessList string, memberName string) error {
	err := a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(a.memberService.WithPrefix(accessList).DeleteResource(ctx, memberName))
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembersForAccessList hard deletes all access list members for an access list.
func (a *AccessListService) DeleteAllAccessListMembersForAccessList(ctx context.Context, accessList string) error {
	err := a.service.RunWhileLocked(ctx, lockName(accessList), accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		_, err := a.service.GetResource(ctx, accessList)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(a.memberService.WithPrefix(accessList).DeleteAllResources(ctx))
	})
	return trace.Wrap(err)
}

// DeleteAllAccessListMembers hard deletes all access list members.
func (a *AccessListService) DeleteAllAccessListMembers(ctx context.Context) error {

	// Locks are not used here as this operation is more likely to be used by the cache.
	return trace.Wrap(a.memberService.DeleteAllResources(ctx))
}

// UpsertAccessListWithMembers creates or updates an access list resource and its members.
func (a *AccessListService) UpsertAccessListWithMembers(ctx context.Context, accessList *accesslist.AccessList, members []*accesslist.AccessListMember) (*accesslist.AccessList, []*accesslist.AccessListMember, error) {
	err := a.service.RunWhileLocked(ctx, lockName(accessList.GetName()), 2*accessListLockTTL, func(ctx context.Context, _ backend.Backend) error {
		membersToken := ""
		membersMap := make(map[string]*accesslist.AccessListMember)
		var err error

		// Convert the members slice to a map for easier lookup.
		for _, member := range members {
			membersMap[member.GetName()] = member
		}

		for {
			members, membersToken, err = a.memberService.WithPrefix(accessList.GetName()).ListResources(ctx, 0 /* default size */, membersToken)
			if err != nil {
				return trace.Wrap(err)
			}

			for _, member := range members {
				// If the member is not in the members map, delete it.
				if _, ok := membersMap[member.GetName()]; !ok {
					err = a.memberService.WithPrefix(accessList.GetName()).DeleteResource(ctx, member.GetName())
					if err != nil {
						return trace.Wrap(err)
					}
				} else {
					// Compare members and update if necessary.
					if !cmp.Equal(member, membersMap[member.GetName()]) {
						err = a.memberService.WithPrefix(accessList.GetName()).UpsertResource(ctx, membersMap[member.GetName()])
						if err != nil {
							return trace.Wrap(err)
						}
					}
				}

				// Remove the member from the map.
				delete(membersMap, member.GetName())
			}

			if membersToken == "" {
				break
			}
		}

		// Add any remaining members to the access list.
		for _, member := range membersMap {
			err = a.memberService.WithPrefix(accessList.GetName()).UpsertResource(ctx, member)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		return trace.Wrap(a.service.UpsertResource(ctx, accessList))
	})

	return accessList, nil, trace.Wrap(err)
}

func lockName(accessListName string) string {
	return strings.Join([]string{"access_list", accessListName}, string(backend.Separator))
}
