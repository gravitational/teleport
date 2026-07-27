package directory

import (
	"maps"
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/mdmsync"
	"github.com/gravitational/teleport/lib/msgraph/msgraphtest"
	"github.com/gravitational/teleport/lib/observability/metrics"
)

func TestMetrics(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("full sync insert collection", func(t *testing.T) {
		storage := msgraphtest.NewDefaultStorage()
		env := newFakeEnv(t, withFakeEnvStorage(storage))
		r, registry := newMeasuredReconciler(t, env)
		_, err := r.Reconcile(t.Context(), mdmsync.SyncModeFull)
		require.NoError(t, err)

		// Reconciliation count
		require.InDelta(t, 1, testutil.ToFloat64(r.metrics.reconciliationCount.WithLabelValues(metricLabelResultSuccess)), 0)
		assertAllEmitted(t, r.metrics, registry, true /* insert collection path */)
	})

	t.Run("full sync reconciler", func(t *testing.T) {
		storage := msgraphtest.NewDefaultStorage()
		env := newFakeEnv(t, withFakeEnvStorage(storage))
		r, registry := newMeasuredReconciler(t, env)

		// Create Access List so the sync goes through the reconciler and not the collection insert.
		aclWithMember := newAccessListWithMembers("acl1", []string{})
		_, err := env.aclSvc.UpsertAccessList(ctx, aclWithMember.AccessList)
		require.NoError(t, err)

		_, err = r.Reconcile(t.Context(), mdmsync.SyncModeFull)
		require.NoError(t, err)

		// Reconciliation count
		require.InDelta(t, 1, testutil.ToFloat64(r.metrics.reconciliationCount.WithLabelValues(metricLabelResultSuccess)), 0)
		assertAllEmitted(t, r.metrics, registry, false /* reconciler path */)
	})

	t.Run("delta sync", func(t *testing.T) {
		defaultStorage := msgraphtest.NewDefaultStorage()
		storage := msgraphtest.NewStorage()
		storage.Applications = defaultStorage.Applications
		env := newFakeEnv(t, withFakeEnvStorage(storage))
		env.cfg.DeltaSyncEnabled = true
		r, registry := newMeasuredReconciler(t, env)

		// Running a full sync sets up delta link required for delta sync.
		_, err := r.Reconcile(t.Context(), mdmsync.SyncModeFull)
		require.NoError(t, err)

		// Reconciliation count
		require.InDelta(t, 1, testutil.ToFloat64(r.metrics.reconciliationCount.WithLabelValues(metricLabelResultSuccess)), 0)

		// Create Access List so the sync goes through the reconciler and not the collection insert.
		aclWithMember := newAccessListWithMembers("acl1", []string{})
		_, err = env.aclSvc.UpsertAccessList(ctx, aclWithMember.AccessList)
		require.NoError(t, err)

		// Create delta diff

		env.fakeGraphServer.SetUsers(slices.Collect(maps.Values(defaultStorage.Users)))
		env.fakeGraphServer.SetGroups(slices.Collect(maps.Values(defaultStorage.Groups)))
		env.fakeGraphServer.SetGroupMembers(msgraphtest.Group1ID, defaultStorage.GroupMembers[msgraphtest.Group1ID])
		env.fakeGraphServer.SetGroupMembers(msgraphtest.Group2ID, defaultStorage.GroupMembers[msgraphtest.Group2ID])
		env.fakeGraphServer.SetGroupMembers(msgraphtest.Group3ID, defaultStorage.GroupMembers[msgraphtest.Group3ID])

		// Run delta sync.
		_, err = r.Reconcile(t.Context(), mdmsync.SyncModePartial)
		require.NoError(t, err)

		// Updated reconciliation count
		require.InDelta(t, 2, testutil.ToFloat64(r.metrics.reconciliationCount.WithLabelValues(metricLabelResultSuccess)), 0)
		assertAllEmitted(t, r.metrics, registry, false /* reconciler path */)

		// Delta-only fetch durations.
		require.GreaterOrEqual(t, histCount(t, r.metrics.reconciliationDuration.WithLabelValues("read_entra_groups_delta")), uint64(1), "read_entra_groups_delta")
		require.GreaterOrEqual(t, histCount(t, r.metrics.reconciliationDuration.WithLabelValues("read_entra_users_delta")), uint64(1), "read_entra_users_delta")
	})
}

// assertAllEmitted checks every configured metrics are emitted.
func assertAllEmitted(t *testing.T, m *directoryMetrics, registry *prometheus.Registry, isInsertCollectionPath bool) {
	t.Helper()

	for _, section := range []string{
		// Time it took to read directory items using Graph API.
		"read_entra_groups",
		"read_entra_members",
		"read_entra_users",

		// Time it took to reconcile discovered items to the backend.
		"reconcile_users",
		"reconcile_access_lists",

		// Total time for all the above operations.
		"total",
	} {
		require.GreaterOrEqual(t, histCount(t, m.reconciliationDuration.WithLabelValues(section)), uint64(1), "section %s", section)
	}

	// Discovered resource counts.
	// Expected resource count as seeded with msgraphtest.NewDefaultStorage().
	require.InDelta(t, 3, testutil.ToFloat64(m.discoveredEntraGroups), 0, "discovered groups")
	require.InDelta(t, 8, testutil.ToFloat64(m.discoveredEntraMemberships), 0, "discovered group members")
	require.InDelta(t, 3, testutil.ToFloat64(m.discoveredEntraUsers), 0, "discovered users")

	// User reconciliation metric emitted by the reconciler.
	// Expects resource count based on "create" event.
	require.InDelta(t, 3, gatherMetricValue(t, registry, "teleport_plugin_entra_id_directory_user_reconciliation_total"), 0)
	// Time it took to reconciler individual users to the backend.
	require.Positive(t, gatherMetricValue(t, registry, "teleport_plugin_entra_id_directory_user_reconciliation_duration_seconds"))

	if isInsertCollectionPath {
		// Return early because the insert collection fast path does not measure individual ACL reconciliation metrics checked below.
		return
	}

	// Time it took to reconciler individual access list with members to the backend.
	require.Positive(t, gatherMetricValue(t, registry, "teleport_plugin_entra_id_directory_accesslist_reconciliation_duration_seconds"))
	require.InDelta(t, 1, testutil.ToFloat64(m.reconciledNestedMemberTotal.WithLabelValues(metricLabelResultSuccess)), 0, "nested member reconciliation")
	require.Positive(t, gatherMetricValue(t, registry, "teleport_plugin_entra_id_directory_accesslist_reconciliation_total"))
	require.GreaterOrEqual(t, histCount(t, m.reconciledNestedMemberDuration), uint64(1), "reconciled nested member duration")
}

func newMeasuredReconciler(t *testing.T, env directoryReconcilerEnv) (*Reconciler, *prometheus.Registry) {
	t.Helper()
	promReg := prometheus.NewRegistry()
	mreg, err := metrics.NewRegistry(promReg, "teleport_plugin", "entra_id_directory") // the actual namespace and subsystem is set by the plugin manager.
	require.NoError(t, err)
	env.cfg.MetricsRegistry = mreg
	r, err := New(env.cfg)
	require.NoError(t, err)
	return r, promReg
}

// histCount returns how many observations a histogram has recorded.
func histCount(t *testing.T, o prometheus.Observer) uint64 {
	t.Helper()

	metric, ok := o.(prometheus.Metric)
	require.True(t, ok, "expected metric")
	var d dto.Metric
	require.NoError(t, metric.Write(&d)) // serialize
	return d.GetHistogram().GetSampleCount()
}

func gatherMetricValue(t *testing.T, reg *prometheus.Registry, name string) float64 {
	t.Helper()

	mfs, err := reg.Gather()
	require.NoError(t, err)
	var total float64
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			switch {
			case m.Histogram != nil:
				// This is needed to assert metrics emitted by collectors that live
				// in another package e.g., services.ReconcilerMetrics which is used
				// to emit user and Access List metrics.
				total += float64(m.GetHistogram().GetSampleCount())
			case m.Counter != nil:
				// Counter works for the metrics emitted from the current directory package.
				total += m.GetCounter().GetValue()
			default:
				// Not a histogram or counter, ignore.
			}
		}
	}
	return total
}
