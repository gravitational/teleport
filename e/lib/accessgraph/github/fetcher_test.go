package github

import (
	"testing"
	"time"

	"github.com/google/go-github/v70/github"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

func Test_convertTokenToProto(t *testing.T) {
	type args struct {
		orgName string
		token   *github.PersonalAccessToken
	}
	testTime := time.Now()
	tests := []struct {
		name string
		args args
		want *accessgraphv1alpha.GithubTokenV1
	}{
		{
			name: "convert token to proto",
			args: args{
				orgName: "test-org",
				token: &github.PersonalAccessToken{
					Permissions: &github.PersonalAccessTokenPermissions{
						Org: map[string]string{"account": "read"},
					},
					AccessGrantedAt: &github.Timestamp{Time: testTime},
					TokenExpired:    toPtr(false),
					TokenExpiresAt:  &github.Timestamp{Time: testTime.Add(24 * time.Hour)},
					TokenID:         toPtr(int64(12345)),
					TokenName:       toPtr("test-token"),
					TokenLastUsedAt: &github.Timestamp{Time: testTime.Add(-1 * time.Hour)},
					Owner: &github.User{
						Login: toPtr("test-user"),
					},
				},
			},
			want: accessgraphv1alpha.GithubTokenV1_builder{
				Name:         "test-token",
				Owner:        "test-user",
				Expires:      timestamppb.New(testTime.Add(24 * time.Hour)),
				Permissions:  []*accessgraphv1alpha.GithubTokenV1Permission{accessgraphv1alpha.GithubTokenV1Permission_builder{Domain: "org", Verb: "read", Object: "account"}.Build()},
				Organization: "test-org",
			}.Build(),
		},
		{
			name: "convert token to proto without permissions",
			args: args{
				orgName: "test-org",
				token: &github.PersonalAccessToken{
					AccessGrantedAt: &github.Timestamp{Time: testTime},
					TokenExpired:    toPtr(false),
					TokenExpiresAt:  &github.Timestamp{Time: testTime.Add(24 * time.Hour)},
					TokenID:         toPtr(int64(12345)),
					TokenName:       toPtr("test-token"),
					TokenLastUsedAt: &github.Timestamp{Time: testTime.Add(-1 * time.Hour)},
					Owner: &github.User{
						Login: toPtr("test-user"),
					},
				},
			},
			want: accessgraphv1alpha.GithubTokenV1_builder{
				Name:         "test-token",
				Owner:        "test-user",
				Expires:      timestamppb.New(testTime.Add(24 * time.Hour)),
				Organization: "test-org",
			}.Build(),
		},
		{
			name: "convert token to proto without expiration",
			args: args{
				orgName: "test-org",
				token: &github.PersonalAccessToken{
					AccessGrantedAt: &github.Timestamp{Time: testTime},
					TokenExpired:    toPtr(false),
					TokenID:         toPtr(int64(12345)),
					TokenName:       toPtr("test-token"),
					TokenLastUsedAt: &github.Timestamp{Time: testTime.Add(-1 * time.Hour)},
					Owner: &github.User{
						Login: toPtr("test-user"),
					},
				},
			},
			want: accessgraphv1alpha.GithubTokenV1_builder{
				Name:         "test-token",
				Owner:        "test-user",
				Organization: "test-org",
			}.Build(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertTokenToProto(tt.args.orgName, tt.args.token)
			require.Equal(t, tt.want, got)
		})
	}
}
