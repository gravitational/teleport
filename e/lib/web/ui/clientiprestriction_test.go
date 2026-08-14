package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestToClientIPRestriction(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		in   *clientiprestrictionv1pb.ClientIPRestriction
		want ClientIPRestriction
	}{
		{
			name: "full resource",
			in: clientiprestrictionv1pb.ClientIPRestriction_builder{
				Metadata: headerv1.Metadata_builder{Revision: "rev-1"}.Build(),
				Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
					AllowedCidrs: []string{"10.0.0.0/8", "192.168.0.0/16"},
					Mode:         "enforced",
					Expires:      timestamppb.New(expires),
				}.Build(),
				Status: clientiprestrictionv1pb.ClientIPRestrictionStatus_builder{
					State: "active",
				}.Build(),
			}.Build(),
			want: ClientIPRestriction{
				Cidrs:    []string{"10.0.0.0/8", "192.168.0.0/16"},
				Mode:     "enforced",
				Expires:  &expires,
				Status:   "active",
				Revision: "rev-1",
			},
		},
		{
			name: "draft without expiry",
			in: clientiprestrictionv1pb.ClientIPRestriction_builder{
				Metadata: headerv1.Metadata_builder{Revision: "rev-2"}.Build(),
				Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
					AllowedCidrs: []string{"10.0.0.0/8"},
					Mode:         "draft",
				}.Build(),
				Status: clientiprestrictionv1pb.ClientIPRestrictionStatus_builder{
					State: "draft",
				}.Build(),
			}.Build(),
			want: ClientIPRestriction{
				Cidrs:    []string{"10.0.0.0/8"},
				Mode:     "draft",
				Status:   "draft",
				Revision: "rev-2",
			},
		},
		{
			name: "empty resource",
			in:   &clientiprestrictionv1pb.ClientIPRestriction{},
			want: ClientIPRestriction{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ToClientIPRestriction(tc.in)
			require.Equal(t, tc.want.Cidrs, got.Cidrs)
			require.Equal(t, tc.want.Mode, got.Mode)
			require.Equal(t, tc.want.Status, got.Status)
			require.Equal(t, tc.want.Revision, got.Revision)
			if tc.want.Expires == nil {
				require.Nil(t, got.Expires)
			} else {
				require.NotNil(t, got.Expires)
				require.True(t, tc.want.Expires.Equal(*got.Expires))
			}
		})
	}
}

func TestPutClientIPRestrictionRequest_ToProto(t *testing.T) {
	t.Parallel()

	expires := time.Date(2026, 7, 22, 12, 30, 0, 0, time.UTC)

	t.Run("maps all fields", func(t *testing.T) {
		req := PutClientIPRestrictionRequest{
			Cidrs:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			Mode:     "enforced",
			Expires:  &expires,
			Revision: "rev-1",
		}

		cir := req.ToProto()
		require.Equal(t, req.Cidrs, cir.GetSpec().GetAllowedCidrs())
		require.Equal(t, "enforced", cir.GetSpec().GetMode())
		require.Equal(t, "rev-1", cir.GetMetadata().GetRevision())
		require.NotNil(t, cir.GetSpec().GetExpires())
		require.True(t, expires.Equal(cir.GetSpec().GetExpires().AsTime()))
	})

	t.Run("nil expiry leaves expires unset", func(t *testing.T) {
		req := PutClientIPRestrictionRequest{
			Cidrs: []string{"10.0.0.0/8"},
			Mode:  "draft",
		}

		cir := req.ToProto()
		require.Nil(t, cir.GetSpec().GetExpires())
		require.Empty(t, cir.GetMetadata().GetRevision())
		require.Equal(t, "draft", cir.GetSpec().GetMode())
	})
}
