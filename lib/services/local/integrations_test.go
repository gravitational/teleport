package local

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestDeleteIntegration_DeletesReferencedExternalCAKeyStorage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bk.Close())
	})

	svc, err := NewIntegrationsService(bk)
	require.NoError(t, err)

	const deletedIntegration = "aws-oidc-to-delete"
	const keptIntegration = "aws-oidc-keep"
	require.NoError(t, createAWSOIDCIntegration(ctx, svc, deletedIntegration))
	require.NoError(t, createAWSOIDCIntegration(ctx, svc, keptIntegration))

	putExternalCAKeyStorageConfig(t, ctx, bk, draftExternalCAKeyStorageBackendKey, deletedIntegration)
	putExternalCAKeyStorageConfig(t, ctx, bk, clusterExternalCAKeyStorageBackendKey, keptIntegration)

	require.NoError(t, svc.DeleteIntegration(ctx, deletedIntegration))

	_, err = svc.GetIntegration(ctx, deletedIntegration)
	require.Error(t, err)

	_, err = bk.Get(ctx, draftExternalCAKeyStorageBackendKey)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err))

	_, err = bk.Get(ctx, clusterExternalCAKeyStorageBackendKey)
	require.NoError(t, err)
}

func TestDeleteIntegration_KeepsUnreferencedExternalCAKeyStorage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	bk, err := memory.New(memory.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, bk.Close())
	})

	svc, err := NewIntegrationsService(bk)
	require.NoError(t, err)

	const deletedIntegration = "aws-oidc-to-delete"
	const referencedIntegration = "aws-oidc-referenced"
	require.NoError(t, createAWSOIDCIntegration(ctx, svc, deletedIntegration))
	require.NoError(t, createAWSOIDCIntegration(ctx, svc, referencedIntegration))

	putExternalCAKeyStorageConfig(t, ctx, bk, draftExternalCAKeyStorageBackendKey, referencedIntegration)
	putExternalCAKeyStorageConfig(t, ctx, bk, clusterExternalCAKeyStorageBackendKey, referencedIntegration)

	require.NoError(t, svc.DeleteIntegration(ctx, deletedIntegration))

	_, err = bk.Get(ctx, draftExternalCAKeyStorageBackendKey)
	require.NoError(t, err)

	_, err = bk.Get(ctx, clusterExternalCAKeyStorageBackendKey)
	require.NoError(t, err)
}

func createAWSOIDCIntegration(ctx context.Context, svc *IntegrationsService, name string) error {
	ig, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: name},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "arn:aws:iam::123456789012:role/TestRole",
		},
	)
	if err != nil {
		return err
	}

	_, err = svc.CreateIntegration(ctx, ig)
	return trace.Wrap(err)
}

func putExternalCAKeyStorageConfig(t *testing.T, ctx context.Context, bk backend.Backend, key backend.Key, integrationName string) {
	t.Helper()

	data, err := json.Marshal(map[string]any{
		"mode": "cluster",
		"spec": map[string]string{
			"integrationName": integrationName,
			"awsAccountID":    "123456789012",
			"awsRegion":       "us-west-2",
			"keyAliasPrefix":  "teleport-ca",
		},
	})
	require.NoError(t, err)

	_, err = bk.Put(ctx, backend.Item{
		Key:   key,
		Value: data,
	})
	require.NoError(t, err)
}
