package accesslist

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func Test_validateMemberRequest(t *testing.T) {
	t.Parallel()
	t.Run("missing member.access_list", func(t *testing.T) {
		req := accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: accesslistv1.Member_builder{
				Header: headerv1.ResourceHeader_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test-name",
					}.Build(),
				}.Build(),
			}.Build(),
		}.Build()
		err := validateMemberRequest(req, memberOptions{})
		require.Error(t, err)
		require.ErrorContains(t, err, "member.spec.access_list field is not set")
	})
	t.Run("missing member.name", func(t *testing.T) {
		req := accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: accesslistv1.Member_builder{
				Spec: accesslistv1.MemberSpec_builder{
					AccessList: "test-access-list",
				}.Build(),
			}.Build(),
		}.Build()
		err := validateMemberRequest(req, memberOptions{})
		require.Error(t, err)
		require.ErrorContains(t, err, "member.header.metadata.name field is not set")
	})
	t.Run("valid", func(t *testing.T) {
		req := accesslistv1.UpsertAccessListMemberRequest_builder{
			Member: accesslistv1.Member_builder{
				Header: headerv1.ResourceHeader_builder{
					Metadata: headerv1.Metadata_builder{
						Name: "test-name",
					}.Build(),
				}.Build(),
				Spec: accesslistv1.MemberSpec_builder{
					AccessList: "test-access-list",
				}.Build(),
			}.Build(),
		}.Build()
		err := validateMemberRequest(req, memberOptions{})
		require.NoError(t, err)
	})
	t.Run("spec.name", func(t *testing.T) {
		t.Run("for static requests can be empty", func(t *testing.T) {
			req := accesslistv1.UpsertAccessListMemberRequest_builder{
				Member: accesslistv1.Member_builder{
					Header: headerv1.ResourceHeader_builder{
						Metadata: headerv1.Metadata_builder{
							Name: "test-name",
						}.Build(),
					}.Build(),
					Spec: accesslistv1.MemberSpec_builder{
						AccessList: "test-access-list",
					}.Build(),
				}.Build(),
			}.Build()
			err := validateMemberRequest(req, memberOptions{
				requireStatic: true,
			})
			require.NoError(t, err)
		})

		t.Run("for static requests must be equal to metadata.name", func(t *testing.T) {
			req := accesslistv1.UpsertAccessListMemberRequest_builder{
				Member: accesslistv1.Member_builder{
					Header: headerv1.ResourceHeader_builder{
						Metadata: headerv1.Metadata_builder{
							Name: "test-name",
						}.Build(),
					}.Build(),
					Spec: accesslistv1.MemberSpec_builder{
						Name:       "different-test-name",
						AccessList: "test-access-list",
					}.Build(),
				}.Build(),
			}.Build()
			err := validateMemberRequest(req, memberOptions{
				requireStatic: true,
			})
			require.Error(t, err)
			require.ErrorContains(t, err, `The values of member.header.metadata.name ("test-name") and member.spec.name ("different-test-name") must match`)
			require.True(t, trace.IsBadParameter(err))
		})

		t.Run("for non-static requests can be whatever for backward-compatibility reasons", func(t *testing.T) {
			req := accesslistv1.UpsertAccessListMemberRequest_builder{
				Member: accesslistv1.Member_builder{
					Header: headerv1.ResourceHeader_builder{
						Metadata: headerv1.Metadata_builder{
							Name: "test-name",
						}.Build(),
					}.Build(),
					Spec: accesslistv1.MemberSpec_builder{
						Name:       "different-test-name-checking-backward-compatibility",
						AccessList: "test-access-list",
					}.Build(),
				}.Build(),
			}.Build()
			err := validateMemberRequest(req, memberOptions{})
			require.NoError(t, err)
		})
	})
}

func Test_validateMemberMetaRequest(t *testing.T) {
	t.Parallel()
	t.Run("missing access_list", func(t *testing.T) {
		req := accesslistv1.GetAccessListMemberRequest_builder{
			MemberName: "test-member-name",
		}.Build()
		err := validateMemberMetaRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "access_list field is not set")
	})
	t.Run("missing member_name", func(t *testing.T) {
		req := accesslistv1.GetAccessListMemberRequest_builder{
			AccessList: "test-access-list",
		}.Build()
		err := validateMemberMetaRequest(req)
		require.Error(t, err)
		require.ErrorContains(t, err, "member_name field is not set")
	})
	t.Run("valid", func(t *testing.T) {
		req := accesslistv1.GetAccessListMemberRequest_builder{
			MemberName: "test-member-name",
			AccessList: "test-access-list",
		}.Build()
		err := validateMemberMetaRequest(req)
		require.NoError(t, err)
	})
}
