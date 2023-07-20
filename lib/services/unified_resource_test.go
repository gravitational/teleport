package services_test

import (
	// "context"
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedResourceWatcher(t *testing.T) {
	t.Parallel()

	// Set up the test.
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	bk, err := memory.New(memory.Config{Context: ctx, Clock: clock})
	require.NoError(t, err)

	presenceSrv := local.NewPresenceService(bk)
	winSrv := local.NewWindowsDesktopService(bk)

	w, err := services.NewUnifiedResourceWatcher(ctx, services.UnifiedResourceWatcherConfig{
		ResourceWatcherConfig: services.ResourceWatcherConfig{
			Component: "test",
			Client:    local.NewEventsService(bk),
		},
		NodesGetter:           presenceSrv,
		DatabaseServersGetter: presenceSrv,
		AppServersGetter:      presenceSrv,
		WindowsDesktopGetter:  winSrv,
	})
	require.NoError(t, err)
	t.Cleanup(w.Close)

	// No resources expected initially.
	res, err := w.GetUnifiedResources(ctx)
	require.NoError(t, err)
	assert.Empty(t, res)

	// Add resources to the backend.
	node := newNodeServer(t, "node1", "127.0.0.1:22", false /*tunnel*/)
	_, err = presenceSrv.UpsertNode(ctx, node)
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
	_, err = presenceSrv.UpsertDatabaseServer(ctx, db)
	require.NoError(t, err)

	app, err := types.NewAppServerV3(
		types.Metadata{Name: "app1"},
		types.AppServerSpecV3{
			HostID: "app1-host-id",
			App:    newApp(t, "app1"),
		},
	)
	require.NoError(t, err)
	_, err = presenceSrv.UpsertApplicationServer(ctx, app)
	require.NoError(t, err)

	win, err := types.NewWindowsDesktopV3(
		"win1",
		nil,
		types.WindowsDesktopSpecV3{Addr: "localhost", HostID: "win1-host-id"},
	)
	require.NoError(t, err)
	err = winSrv.UpsertWindowsDesktop(ctx, win)
	require.NoError(t, err)

	expectedRes := []types.ResourceWithLabels{node, db, app, win}
	assert.Eventually(t, func() bool {
		res, err = w.GetUnifiedResources(ctx)
		require.NoError(t, err)
		return len(res) == len(expectedRes)
	}, time.Second, time.Millisecond, "Timed out waiting for unified resources")
	assert.Empty(t, cmp.Diff(
		expectedRes,
		res,
		cmpopts.EquateEmpty(),
		cmpopts.IgnoreFields(types.Metadata{}, "ID"),
		// Ignore order.
		cmpopts.SortSlices(func(a, b types.ResourceWithLabels) bool { return a.GetName() < b.GetName() }),
	))
}
