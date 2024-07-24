package igs

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/lib/services"
)

func isAccessListOwner(acl *accesslist.AccessList, user string) bool {
	found := slices.IndexFunc(acl.GetOwners(), func(v accesslist.Owner) bool {
		return v.Name == user
	}) != -1
	return found
}

func isAccessListMember(members []*accesslist.AccessListMember, user string) bool {
	found := slices.IndexFunc(members, func(v *accesslist.AccessListMember) bool {
		return v.Spec.Name == user
	}) != -1
	return found
}

func mustGetAccessListAndMembers(t *testing.T, client services.AccessLists, name string) (*accesslist.AccessList, []*accesslist.AccessListMember) {
	acl := mustGetAccessList(t, client, name)
	members := mustGetAccessListMembers(t, client, name)
	return acl, members
}

func mustGetAccessList(t *testing.T, client services.AccessLists, name string) *accesslist.AccessList {
	out, err := client.GetAccessList(context.Background(), name)
	require.NoError(t, err)
	return out

}

func mustGetAccessListMembers(t *testing.T, client services.AccessLists, name string) []*accesslist.AccessListMember {
	var out, members []*accesslist.AccessListMember
	var err error
	var token string
	for {
		members, token, err = client.ListAccessListMembers(context.Background(), name, 0, "")
		require.NoError(t, err)
		out = append(out, members...)
		if token == "" {
			break
		}
	}
	return out
}

func mustCreateMember(t *testing.T, aclName, memberName string) *accesslist.AccessListMember {
	clock := clockwork.NewRealClock()
	member, err := accesslist.NewAccessListMember(
		header.Metadata{
			Name: memberName,
		},
		accesslist.AccessListMemberSpec{
			AccessList: aclName,
			Name:       memberName,
			Joined:     clock.Now(),
			AddedBy:    "added by",
			Expires:    clock.Now().Add(time.Hour * 24).UTC(),
		},
	)
	require.NoError(t, err)
	return member
}
