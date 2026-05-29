/*
Copyright 2026 Gravitational, Inc.

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

package main

import (
	"context"

	"github.com/gravitational/trace"

	accesslistclient "github.com/gravitational/teleport/api/client/accesslist"
	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

// isManagedBySync returns true for Access Lists that were created by this sync
// tool. The reconciler uses this as its Matcher so it never touches lists that
// were created manually.
func isManagedBySync(al *accesslist.AccessList) bool {
	return al.GetMetadata().Labels[managedByLabel] == managedByValue
}

// listAccessListsFromTeleport pages through all Access Lists in Teleport via
// ListAccessListsV2 and returns only the ones managed by this sync tool.
func listAccessListsFromTeleport(ctx context.Context, alClient *accesslistclient.Client) (map[string]*accesslist.AccessList, error) {
	listPage := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
		return alClient.ListAccessListsV2(ctx, &accesslistv1.ListAccessListsV2Request{
			PageSize:  int32(pageSize),
			PageToken: pageToken,
		})
	}
	all, err := stream.Collect(clientutils.Resources(ctx, listPage))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make(map[string]*accesslist.AccessList)
	for _, al := range all {
		if isManagedBySync(al) {
			out[al.GetName()] = al
		}
	}
	return out, nil
}

// accessListMembersFromTeleport pages through all members of the named Access
// Lists and returns them keyed by "<access-list-name>/<username>".
func accessListMembersFromTeleport(ctx context.Context, names []string, alClient *accesslistclient.Client) (map[string]*accesslist.AccessListMember, error) {
	out := map[string]*accesslist.AccessListMember{}
	for _, name := range names {
		listMembers := func(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessListMember, string, error) {
			return alClient.ListAccessListMembers(ctx, name, pageSize, pageToken)
		}
		members, err := stream.Collect(clientutils.Resources(ctx, listMembers))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, m := range members {
			out[memberMapKey(m)] = m
		}
	}
	return out, nil
}

func memberMapKey(m *accesslist.AccessListMember) string {
	return m.Spec.AccessList + "/" + m.Spec.Name
}

func accessListNames(in map[string]*accesslist.AccessList) []string {
	out := make([]string, 0, len(in))
	for name := range in {
		out = append(out, name)
	}
	return out
}

// newAccessListReconciler builds a reconciler that creates, updates, and
// deletes Access Lists to match the incoming map. isManagedBySync is used as
// the Matcher so manually-created lists are never touched.
func newAccessListReconciler(alClient *accesslistclient.Client, existing, incoming map[string]*accesslist.AccessList) (*services.Reconciler[*accesslist.AccessList], error) {
	return services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessList]{
		Matcher: isManagedBySync,
		CompareResources: func(a, b *accesslist.AccessList) int {
			return services.EqualFromBool(accesslist.EqualAccessLists(a, b, accesslist.WithIgnoreEphemeralFields()))
		},
		GetCurrentResources: func() map[string]*accesslist.AccessList { return existing },
		GetNewResources:     func() map[string]*accesslist.AccessList { return incoming },
		OnCreate: func(ctx context.Context, al *accesslist.AccessList) error {
			_, err := alClient.UpsertAccessList(ctx, al)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming, _ *accesslist.AccessList) error {
			_, err := alClient.UpsertAccessList(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, al *accesslist.AccessList) error {
			return trace.Wrap(alClient.DeleteAccessList(ctx, al.GetName()))
		},
	})
}

// newAccessListMemberReconciler builds a reconciler that creates, updates, and
// deletes Access List members to match the incoming map.
func newAccessListMemberReconciler(alClient *accesslistclient.Client, existing, incoming map[string]*accesslist.AccessListMember) (*services.Reconciler[*accesslist.AccessListMember], error) {
	return services.NewReconciler(services.ReconcilerConfig[*accesslist.AccessListMember]{
		Matcher: func(_ *accesslist.AccessListMember) bool { return true },
		CompareResources: func(a, b *accesslist.AccessListMember) int {
			if a.Spec.Name == b.Spec.Name && a.Spec.AccessList == b.Spec.AccessList {
				return services.Equal
			}
			return services.Different
		},
		GetCurrentResources: func() map[string]*accesslist.AccessListMember { return existing },
		GetNewResources:     func() map[string]*accesslist.AccessListMember { return incoming },
		OnCreate: func(ctx context.Context, m *accesslist.AccessListMember) error {
			_, err := alClient.UpsertAccessListMember(ctx, m)
			return trace.Wrap(err)
		},
		OnUpdate: func(ctx context.Context, incoming, _ *accesslist.AccessListMember) error {
			_, err := alClient.UpsertAccessListMember(ctx, incoming)
			return trace.Wrap(err)
		},
		OnDelete: func(ctx context.Context, m *accesslist.AccessListMember) error {
			err := alClient.DeleteAccessListMember(ctx, m.Spec.AccessList, m.Spec.Name)
			if trace.IsNotFound(err) {
				return nil
			}
			return trace.Wrap(err)
		},
	})
}
