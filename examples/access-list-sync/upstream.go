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

	"github.com/gravitational/teleport/api/types/accesslist"
)

// managedByLabel is stamped on every Access List created by this sync tool so
// the matcher can distinguish them from manually-managed lists.
const managedByLabel = "managed-by"
const managedByValue = "external-sync"

// Group is a group returned by the upstream identity system.
type Group struct {
	ID          string
	DisplayName string
}

// Member is a member of a group in the upstream identity system.
type Member struct {
	UserName string
}

// GroupWithMembers pairs a group with its member list.
type GroupWithMembers struct {
	Group
	Members []Member
}

// UpstreamClient is the interface for fetching data from the external system.
type UpstreamClient interface {
	ListGroups(ctx context.Context) ([]Group, error)
	ListGroupsWithMembers(ctx context.Context) ([]GroupWithMembers, error)
}

// mockUpstreamClient returns a single hardcoded group with one member,
// simulating what a real upstream identity provider would return.
type mockUpstreamClient struct{}

func (m *mockUpstreamClient) ListGroups(_ context.Context) ([]Group, error) {
	return []Group{
		{ID: "eng-platform", DisplayName: "Engineering Platform"},
	}, nil
}

func (m *mockUpstreamClient) ListGroupsWithMembers(_ context.Context) ([]GroupWithMembers, error) {
	return []GroupWithMembers{
		{
			Group:   Group{ID: "eng-platform", DisplayName: "Engineering Platform"},
			Members: []Member{{UserName: "alice"}},
		},
	}, nil
}

// accessListsFromUpstream queries the upstream source and converts each group
// into an accesslist.AccessList keyed by the group's stable ID.
func accessListsFromUpstream(ctx context.Context, upstream UpstreamClient) (map[string]*accesslist.AccessList, error) {
	groups, err := upstream.ListGroups(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := make(map[string]*accesslist.AccessList, len(groups))
	for _, g := range groups {
		acl, err := groupToAccessList(g, toOwners("alice", "bob")) // replace with real owner list
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[acl.GetName()] = acl
	}
	return out, nil
}

// accessListMembersFromUpstream fetches group memberships from the upstream
// source and returns them keyed by "<access-list-name>/<username>".
func accessListMembersFromUpstream(ctx context.Context, upstream UpstreamClient) (map[string]*accesslist.AccessListMember, error) {
	groups, err := upstream.ListGroupsWithMembers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	out := map[string]*accesslist.AccessListMember{}
	for _, g := range groups {
		for _, m := range g.Members {
			alm, err := memberToAccessListMember(g.ID, m)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out[memberMapKey(alm)] = alm
		}
	}
	return out, nil
}
