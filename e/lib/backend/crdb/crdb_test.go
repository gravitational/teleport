package crdb

import (
	"context"
	"encoding/json"
	"os"
	"testing"

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
