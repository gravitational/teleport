package integration

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

func mustGetAccessListAndMembers(t require.TestingT, client services.AccessLists, name string) (*accesslist.AccessList, []*accesslist.AccessListMember) {
	callHelper(t)

	acl := mustGetAccessList(t, client, name)
	members := mustGetAllAccessListMembers(t, client, name)
	return acl, members
}

func mustGetAccessList(t require.TestingT, client services.AccessLists, name string) *accesslist.AccessList {
	callHelper(t)
	ctx, cancel := getContext(t, 10*time.Second)
	defer cancel()

	out, err := client.GetAccessList(ctx, name)
	require.NoError(t, err)
	return out
}

func mustGetAccessListMember(t require.TestingT, client services.AccessLists, accessList, member string) *accesslist.AccessListMember {
	callHelper(t)
	ctx, cancel := getContext(t, 10*time.Second)
	defer cancel()

	res, err := client.GetAccessListMember(ctx, accessList, member)
	require.NoError(t, err)
	return res
}

func mustGetAllAccessListMembers(t require.TestingT, client services.AccessLists, name string) []*accesslist.AccessListMember {
	callHelper(t)
	ctx, cancel := getContext(t, 30*time.Second)
	defer cancel()

	var out, members []*accesslist.AccessListMember
	var err error
	var token string
	for {
		members, token, err = client.ListAccessListMembers(ctx, name, 0, "")
		require.NoError(t, err)
		out = append(out, members...)
		if token == "" {
			break
		}
	}
	return out
}

func mustCreateMember(t *testing.T, aclName, memberName string) *accesslist.AccessListMember {
	t.Helper()
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

func callHelper(t require.TestingT) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
}

func getContext(t require.TestingT, timeout time.Duration) (ctx context.Context, cancel func()) {
	if c, ok := t.(interface{ Context() context.Context }); ok {
		ctx = c.Context()
	} else {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, timeout)
}
