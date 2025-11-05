package conv

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/e/lib/scim/service/common"
)

func TestUserConv(t *testing.T) {
	attrs, err := structpb.NewStruct(map[string]any{
		common.UsernameAttribute: "alice@example.com",
	})
	require.NoError(t, err)
	want := &scimpb.Resource{
		Id: "alice@example.com",
		Meta: &scimpb.Meta{
			Version:      `W/"version1"`,
			ResourceType: common.ResourceTypeUser,
		},
		ExternalId: "external-id",
		Attributes: attrs,
	}
	labels := map[string]string{
		"external-id": want.GetExternalId(),
	}
	u, err := UserFromResource(want, WithLabels(labels))
	require.NoError(t, err)

	require.Equal(t, "alice@example.com", u.GetName())
	require.Equal(t, `version1`, u.GetRevision())

	got, err := UserToResource(u, WithExternalIDFunc(func(u types.User) string {
		v, _ := u.GetLabel("external-id")
		return v
	}))
	require.NoError(t, err)
	require.NotNil(t, got.GetMeta().GetCreated())
	got.Meta.Created = nil
	require.Equal(t, want, got)
}

func TestGroupConv(t *testing.T) {
	attrs, err := structpb.NewStruct(map[string]any{
		"displayName": "Access List 1",
		"members": []any{
			map[string]any{
				"value":   "user1",
				"display": "user1",
			},
			map[string]any{
				"value":   "user2",
				"display": "user2",
			},
		},
	})
	require.NoError(t, err)
	want := &scimpb.Resource{
		Meta: &scimpb.Meta{
			Version:      `W/"version1"`,
			ResourceType: common.ResourceTypeGroup,
		},
		Attributes: attrs,
	}
	labels := map[string]string{"test": "test"}
	grants := accesslist.Grants{Roles: []string{"admin", "developer"}}
	acl, members, err := AccessListFromResource(want,
		WithAccessListLabels(labels),
		WithGrants(grants),
	)
	require.NoError(t, err)

	require.Equal(t, "Access List 1", acl.Spec.Title)
	require.Equal(t, "version1", acl.GetRevision())
	require.Equal(t, labels, acl.GetStaticLabels())
	require.Equal(t, grants, acl.GetGrants())
	require.Len(t, members, 2)
	require.Equal(t, "user1", members[0].GetName())
	require.Equal(t, "user2", members[1].GetName())

	got, err := AccessListToResource(acl, members)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
