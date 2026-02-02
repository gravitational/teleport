package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/e/tests/common"
)

func TestAccessListWithPreset(t *testing.T) {
	ctx := t.Context()
	sut := common.InitSUT(t,
		common.WithLicense("../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-editor", "editor"),
	)

	webClient := sut.CreateWebClientForUser(t, "alice-editor")

	t.Run("test preset access list creation", func(t *testing.T) {
		req := ui.AccessListWithPresetRequest{
			PresetType: "long-term",
			AccessList: createValidAccessList(t, "prod-access-list"),
		}
		// call POST /enterprise/accesslist/preset
		endpoint := webClient.Endpoint("enterprise", "accesslistpreset")
		_, err := common.Roundtrip[ui.AccessListWithPresetResponse](ctx, webClient, http.MethodPost, endpoint, req)
		require.Error(t, err)
		require.True(t, trace.IsNotImplemented(err))

	})

	t.Run("test preset access list update", func(t *testing.T) {
		accessListID := "test-id"
		req := ui.AccessListWithPresetRequest{
			PresetType: "long-term",
			AccessList: createValidAccessList(t, accessListID),
		}
		// call PUT /enterprise/accesslistpreset/:accessListId
		endpoint := webClient.Endpoint("enterprise", "accesslistpreset", accessListID)
		_, err := common.Roundtrip[ui.AccessListWithPresetResponse](ctx, webClient, http.MethodPut, endpoint, req)
		require.Error(t, err)
		require.True(t, trace.IsNotImplemented(err))
	})

	t.Run("test preset access list deletion", func(t *testing.T) {
		// call DELETE /enterprise/accesslistpreset/:accessListId
		endpoint := webClient.Endpoint("enterprise", "accesslistpreset", "test-id")
		_, err := common.Roundtrip[any](ctx, webClient, http.MethodDelete, endpoint, nil)
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err))
	})
}

func createValidAccessList(t *testing.T, name string) *ui.AccessList {
	al, err := accesslist.NewAccessList(header.Metadata{
		Name: name,
	}, accesslist.Spec{
		Title: "Test Access List",
		Grants: accesslist.Grants{
			Roles: []string{"access"},
		},
		Audit: accesslist.Audit{
			NextAuditDate: time.Now().AddDate(1, 0, 0),
		},
		Owners: []accesslist.Owner{
			{
				Name:           "alice-editor",
				MembershipKind: accesslist.MembershipKindUser,
			},
		},
	})
	require.NoError(t, err)

	return &ui.AccessList{
		AccessList: al,
	}
}
