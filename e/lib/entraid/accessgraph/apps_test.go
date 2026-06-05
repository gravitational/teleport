package accessgraph

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph/models"
)

func TestEntraAppToProto(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewString()
	appID := uuid.NewString()
	tenantID := uuid.NewString()
	displayName := "My app"
	federatedSSOV2 := gzipBytes([]byte("federatedSSOV2_payload"))
	certs := []string{"cert1", "cert2"}
	ssoSettings := &types.PluginEntraIDAppSSOSettings{
		FederatedSsoV2: federatedSSOV2,
	}

	createEntraApp := func() *models.Application {
		entraApp := &models.Application{}
		entraApp.ID = &id
		entraApp.AppID = &appID
		entraApp.DisplayName = &displayName
		return entraApp
	}

	t.Run("valid", func(t *testing.T) {
		entraApp := createEntraApp()
		expected := accessgraphv1alpha.EntraApplication_builder{
			Id:                  id,
			AppId:               appID,
			DisplayName:         displayName,
			TenantId:            tenantID,
			SigningCertificates: certs,
			FederatedSsoV2:      "federatedSSOV2_payload",
		}.Build()
		result, err := entraAppToProto(ctx, entraApp, ssoSettings, tenantID, certs)
		require.NoError(t, err)
		require.Empty(t, cmp.Diff(
			expected, result,
			protocmp.Transform(),
		),
		)
	})

	t.Run("without ID", func(t *testing.T) {
		entraApp := createEntraApp()
		entraApp.ID = nil
		_, err := entraAppToProto(ctx, entraApp, ssoSettings, tenantID, certs)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("without app ID", func(t *testing.T) {
		entraApp := createEntraApp()
		entraApp.AppID = nil
		_, err := entraAppToProto(ctx, entraApp, ssoSettings, tenantID, certs)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("without display name", func(t *testing.T) {
		entraApp := createEntraApp()
		entraApp.DisplayName = nil
		_, err := entraAppToProto(ctx, entraApp, ssoSettings, tenantID, certs)
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err))
	})
}

// gzipBytes compresses the given byte slice, returning the result as a new byte slice.
func gzipBytes(src []byte) []byte {
	out := new(bytes.Buffer)
	writer := gzip.NewWriter(out)

	_, err := io.Copy(writer, bytes.NewReader(src))
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}

	err = writer.Close()
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}
	return out.Bytes()
}
