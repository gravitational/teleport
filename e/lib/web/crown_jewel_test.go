package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	crownjewelv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/crownjewel/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
)

func TestCreateCrownJewel(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	webPack := s.newAuthWebPack(t, "foo")
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()
	generateEndpoint := webPack.clt.Endpoint("enterprise", "crownjewels")
	resp, err := webPack.clt.PostJSON(ctx, generateEndpoint, createCrownJewelRequest{
		TeleportMatcher: &crownjewelv1.TeleportMatcher{
			Kinds: []string{"ssh"},
			Name:  "test",
		},
	})
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.Code())

	authResp, _, err := authClient.CrownJewelServiceClient().ListCrownJewels(ctx, 0 /* default limit */, "")
	require.NoError(t, err)

	require.Len(t, authResp, 1)
	require.Equal(t, "test", authResp[0].Spec.TeleportMatchers[0].Name)
}

func TestDeleteCrownJewel(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t,
		// Disable retry interval to prevent test from hanging
		// because it uses the fake clock.
		withRunWhileLockedRetryInterval(-1*time.Millisecond),
	)
	authClient := s.newAdminAuthClient(s.ctx, t)

	ctx := context.Background()
	_, err := authClient.CrownJewelServiceClient().CreateCrownJewel(ctx, &crownjewelv1.CrownJewel{
		Metadata: &headerv1.Metadata{
			Name: "test",
		},
		Spec: &crownjewelv1.CrownJewelSpec{
			TeleportMatchers: []*crownjewelv1.TeleportMatcher{
				{
					Kinds: []string{"ssh"},
					Name:  "test",
				},
			},
		},
	})
	require.NoError(t, err)

	const crownJewelName = "test"
	webPack := s.newAuthWebPack(t, "foo")
	deleteEndpoint := webPack.clt.Endpoint("enterprise", "crownjewels", crownJewelName)
	resp, err := webPack.clt.Delete(ctx, deleteEndpoint)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, resp.Code())

	authResp, _, err := authClient.CrownJewelServiceClient().ListCrownJewels(ctx, 0 /* default limit */, "")
	require.NoError(t, err)

	require.Empty(t, authResp)
}
