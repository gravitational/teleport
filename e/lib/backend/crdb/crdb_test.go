package crdb

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func getParamString(t *testing.T) string {
	paramString := os.Getenv("TELEPORT_CRDB_TEST_PARAMS_JSON")
	if paramString == "" {
		t.Skip("CockroachDB backend tests are disabled. Enable them by setting the TELEPORT_CRDB_TEST_PARAMS_JSON variable.")
	}
	return paramString
}

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestTTLCronJob(t *testing.T) {
	paramString := getParamString(t)

	var (
		params backend.Params
		cfg    Config
	)
	ctx := context.Background()

	require.NoError(t, json.Unmarshal([]byte(paramString), &params))
	if err := utils.ObjectToStruct(params, &cfg); err != nil {
		require.NoError(t, err)
	}

	cfg.TTLJobCron = "20 * * * *"
	bk, err := newFromConfig(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, bk.Close())

	cfg.TTLJobCron = "invalid"
	_, err = newFromConfig(ctx, cfg)
	require.Error(t, err)
}

func TestCockroachDBBackend(t *testing.T) {
	paramString := getParamString(t)
	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}
		var params backend.Params
		require.NoError(t, json.Unmarshal([]byte(paramString), &params))

		ctx := context.Background()
		bk, err := NewFromParams(ctx, params)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return bk, test.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

// TestPutBatchSingleConnPool verifies that PutBatch works correctly when the
// connection pool has only a single connection. Before the fix, PutBatch called
// b.pool.Exec inside b.pool.AcquireFunc, which would deadlock with
// pool_max_conns=1 because the only connection is already held by AcquireFunc
// and pool.Exec blocks trying to acquire another one.
func TestPutBatchSingleConnPool(t *testing.T) {
	paramString := getParamString(t)

	var params backend.Params
	require.NoError(t, json.Unmarshal([]byte(paramString), &params))

	var cfg Config
	require.NoError(t, utils.ObjectToStruct(params, &cfg))

	// Force pool_max_conns=1 to expose the deadlock when Exec is called on
	// the pool instead of the acquired connection.
	u, err := url.Parse(cfg.ConnString)
	require.NoError(t, err)
	q := u.Query()
	q.Set("pool_max_conns", "1")
	u.RawQuery = q.Encode()
	cfg.ConnString = u.String()

	ctx := context.Background()
	bk, err := newFromConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = bk.Close() })

	items := []backend.Item{
		{Key: backend.NewKey("test", "putbatch", "a"), Value: []byte("A"), Expires: time.Now().Add(time.Minute)},
		{Key: backend.NewKey("test", "putbatch", "b"), Value: []byte("B"), Expires: time.Now().Add(time.Minute)},
		{Key: backend.NewKey("test", "putbatch", "c"), Value: []byte("C"), Expires: time.Now().Add(time.Minute)},
	}

	revisions, err := bk.PutBatch(ctx, items)
	require.NoError(t, err)
	require.Len(t, revisions, len(items))

	// Verify the items were actually written.
	for _, item := range items {
		got, err := bk.Get(ctx, item.Key)
		require.NoError(t, err)
		require.Equal(t, item.Value, got.Value)
	}
}
