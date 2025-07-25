package accesslist

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func Test_validateMemberRequest(t *testing.T) {
	t.Run("missing member.access_list", func(t *testing.T) {
		req := &accesslistv1.UpsertAccessListMemberRequest{
			Member: &accesslistv1.Member{
				Header: &headerv1.ResourceHeader{
					Metadata: &headerv1.Metadata{
						Name: "test-name",
					},
				},
			},
		}
		err := validateMemberRequest(req, memberOptions{})
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
		err := validateMemberRequest(req, memberOptions{})
		require.Error(t, err)
		require.ErrorContains(t, err, "member.header.metadata.name field is not set")
	})
	t.Run("valid", func(t *testing.T) {
		req := &accesslistv1.UpsertAccessListMemberRequest{
			Member: &accesslistv1.Member{
				Header: &headerv1.ResourceHeader{
					Metadata: &headerv1.Metadata{
						Name: "test-name",
					},
				},
				Spec: &accesslistv1.MemberSpec{
					AccessList: "test-access-list",
				},
			},
		}
		err := validateMemberRequest(req, memberOptions{})
		require.NoError(t, err)
	})
	t.Run("spec.name", func(t *testing.T) {
		t.Run("for static requests can be empty", func(t *testing.T) {
			req := &accesslistv1.UpsertAccessListMemberRequest{
				Member: &accesslistv1.Member{
					Header: &headerv1.ResourceHeader{
						Metadata: &headerv1.Metadata{
							Name: "test-name",
						},
					},
					Spec: &accesslistv1.MemberSpec{
						AccessList: "test-access-list",
					},
				},
			}
			err := validateMemberRequest(req, memberOptions{
				requireStatic: true,
			})
			require.NoError(t, err)
		})

		t.Run("for static requests must be equal to metadata.name", func(t *testing.T) {
			req := &accesslistv1.UpsertAccessListMemberRequest{
				Member: &accesslistv1.Member{
					Header: &headerv1.ResourceHeader{
						Metadata: &headerv1.Metadata{
							Name: "test-name",
						},
					},
					Spec: &accesslistv1.MemberSpec{
						Name:       "different-test-name",
						AccessList: "test-access-list",
					},
				},
			}
			err := validateMemberRequest(req, memberOptions{
				requireStatic: true,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, `The values of member.header.metadata.name ("test-name") and member.spec.name ("different-test-name") must match`)
			require.True(t, trace.IsBadParameter(err))
		})

		t.Run("for non-static requests can be whatever for backward-compatibility reasons", func(t *testing.T) {
			req := &accesslistv1.UpsertAccessListMemberRequest{
				Member: &accesslistv1.Member{
					Header: &headerv1.ResourceHeader{
						Metadata: &headerv1.Metadata{
							Name: "test-name",
						},
					},
					Spec: &accesslistv1.MemberSpec{
						Name:       "different-test-name-checking-backward-compatibility",
						AccessList: "test-access-list",
					},
				},
			}
			err := validateMemberRequest(req, memberOptions{})
			require.NoError(t, err)
		})
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
