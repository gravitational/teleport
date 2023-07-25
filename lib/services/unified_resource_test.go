package services_test

import (
	// "context"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func setupConfig(t *testing.T, ctx context.Context) (auth.InitConfig, *memory.Memory) {
	tempDir := t.TempDir()

	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{Context: ctx, Clock: clock})
	require.NoError(t, err)

	clusterName, err := services.NewClusterNameWithRandomID(types.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	require.NoError(t, err)

	return auth.InitConfig{
		DataDir:                 tempDir,
		HostUUID:                "00000000-0000-0000-0000-000000000000",
		NodeName:                "foo",
		Backend:                 bk,
		Authority:               testauthority.New(),
		ClusterAuditConfig:      types.DefaultClusterAuditConfig(),
		ClusterNetworkingConfig: types.DefaultClusterNetworkingConfig(),
		SessionRecordingConfig:  types.DefaultSessionRecordingConfig(),
		ClusterName:             clusterName,
		StaticTokens:            types.DefaultStaticTokens(),
		AuthPreference:          types.DefaultAuthPreference(),
		SkipPeriodicOperations:  true,
		KeyStoreConfig: keystore.Config{
			Software: keystore.SoftwareConfig{
				RSAKeyPairSource: testauthority.New().GenerateKeyPair,
			},
		},
		Tracer: tracing.NoopTracer(teleport.ComponentAuth),
	}, bk
}
func TestUnifiedResourceWatcher(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	conf, bk := setupConfig(t, ctx)
	authServer, err := auth.Init(ctx, conf)
	require.NoError(t, err)

	w, err := services.NewUnifiedResourceWatcher(ctx, services.UnifiedResourceWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    local.NewEventsService(bk),
		},
		NodesGetter:                  authServer,
		DatabaseServersGetter:        authServer,
		AppServersGetter:             authServer,
		KubernetesClusterGetter:      authServer,
		WindowsDesktopGetter:         authServer,
		SAMLIdpServiceProviderGetter: authServer,
	})
	require.NoError(t, err)
	t.Cleanup(func() { w.Close() })

	// No resources expected initially.
	res, err := w.GetUnifiedResources(ctx)
	require.NoError(t, err)
	assert.Empty(t, res)

	// Add node to the backend.
	node := newNodeServer(t, "node1", "127.0.0.1:22", false /*tunnel*/)
	_, err = authServer.UpsertNode(ctx, node)
	require.NoError(t, err)

	db, err := types.NewDatabaseServerV3(
		types.Metadata{Name: "db1"},
		types.DatabaseServerSpecV3{
			Protocol: "postgres",
			Hostname: "localhost",
			HostID:   "db1-host-id",
		},
	)
	require.NoError(t, err)
	_, err = authServer.UpsertDatabaseServer(ctx, db)
	require.NoError(t, err)

	// Add app to the backend.
	app, err := types.NewAppServerV3(
		types.Metadata{Name: "app1"},
		types.AppServerSpecV3{
			HostID: "app1-host-id",
			App:    newApp(t, "app1"),
		},
	)
	require.NoError(t, err)
	_, err = authServer.UpsertApplicationServer(ctx, app)
	require.NoError(t, err)

	// Add saml idp service provider to the backend.
	samlapp, err := types.NewSAMLIdPServiceProvider(
		types.Metadata{
			Name: "sp1",
		},
		types.SAMLIdPServiceProviderSpecV1{
			EntityDescriptor: newTestEntityDescriptor("sp1"),
			EntityID:         "sp1",
		},
	)
	require.NoError(t, err)
	err = authServer.CreateSAMLIdPServiceProvider(ctx, samlapp)
	require.NoError(t, err)

	win, err := types.NewWindowsDesktopV3(
		"win1",
		nil,
		types.WindowsDesktopSpecV3{Addr: "localhost", HostID: "win1-host-id"},
	)
	require.NoError(t, err)
	err = authServer.UpsertWindowsDesktop(ctx, win)
	require.NoError(t, err)

	// we expect each of the resources above to exist
	expectedRes := []types.ResourceWithLabels{node, app, samlapp, db, win}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		return len(res) == len(expectedRes)
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be added")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))

	// // Update and remove some resources.
	nodeUpdated := newNodeServer(t, "node1", "192.168.0.1:22", false /*tunnel*/)
	_, err = authServer.UpsertNode(ctx, nodeUpdated)
	require.NoError(t, err)
	err = authServer.DeleteApplicationServer(ctx, defaults.Namespace, "app1-host-id", "app1")
	require.NoError(t, err)

	// this should include the updated node, and shouldn't have any apps included
	expectedRes = []types.ResourceWithLabels{nodeUpdated, samlapp, db, win}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		require.NoError(t, err)
		serverUpdated := slices.ContainsFunc(res, func(r types.ResourceWithLabels) bool {
			node, ok := r.(types.Server)
			return ok && node.GetAddr() == "192.168.0.1:22"
		})
		return len(res) == len(expectedRes) && serverUpdated
	}, 5*time.Second, 10*time.Millisecond, "Timed out waiting for unified resources to be updated")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))
}

func newTestEntityDescriptor(entityID string) string {
	return fmt.Sprintf(testEntityDescriptor, entityID)
}

// A test entity descriptor from https://sptest.iamshowcase.com/testsp_metadata.xml.
const testEntityDescriptor = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="%s" validUntil="2025-12-09T09:13:31.006Z">
   <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified</md:NameIDFormat>
      <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
      <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sptest.iamshowcase.com/acs" index="0" isDefault="true"/>
   </md:SPSSODescriptor>
</md:EntityDescriptor>
`
