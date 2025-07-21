package accesslist

import (
	"testing"

	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
)

func Test_validateMemberRequest(t *testing.T) {
	t.Run("missing member.access_list", func(t *testing.T) {
		req := &accesslistv1.UpsertAccessListMemberRequest{
			Member: &accesslistv1.Member{
				Spec: &accesslistv1.MemberSpec{
					Name: "test-name",
				},
			},
		}
		err := validateMemberRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "member.spec.access_list field is not set")
	})
	t.Run("missing member.name", func(t *testing.T) {
		req := &accesslistv1.UpsertAccessListMemberRequest{
			Member: &accesslistv1.Member{
				Spec: &accesslistv1.MemberSpec{
					AccessList: "test-access-list",
				},
			},
		}
		err := validateMemberRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "member.spec.name field is not set")
	})
	t.Run("valid", func(t *testing.T) {
		req := &accesslistv1.UpsertAccessListMemberRequest{
			Member: &accesslistv1.Member{
				Spec: &accesslistv1.MemberSpec{
					Name:       "test-name",
					AccessList: "test-access-list",
				},
			},
		}
		err := validateMemberRequest(req)
		require.NoError(t, err)
	})
}

func Test_validateMemberMetaRequest(t *testing.T) {
	t.Run("missing access_list", func(t *testing.T) {
		req := &accesslistv1.GetAccessListMemberRequest{
			MemberName: "test-member-name",
		}
		err := validateMemberMetaRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "access_list field is not set")
	})
	t.Run("missing member_name", func(t *testing.T) {
		req := &accesslistv1.GetAccessListMemberRequest{
			AccessList: "test-access-list",
		}
		err := validateMemberMetaRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "member_name field is not set")
	})
	t.Run("valid", func(t *testing.T) {
		req := &accesslistv1.GetAccessListMemberRequest{
			MemberName: "test-member-name",
			AccessList: "test-access-list",
		}
		err := validateMemberMetaRequest(req)
		require.NoError(t, err)
	})
}
