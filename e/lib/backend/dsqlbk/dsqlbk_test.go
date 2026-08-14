package dsqlbk_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/e/lib/backend/dsqlbk"
	"github.com/gravitational/teleport/lib/backend"
	backendtest "github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func checkTestParams(t testing.TB) {
	t.Helper()

	// expiry_interval needs to be really short to pass some of the tests, and a
	// faster poll interval helps a bit with runtime:
	//
	// {"endpoint":"...","user":"...","kinesis_stream":"...","aws_region":"...","expiry_interval":"500ms"}

	if os.Getenv("TELEPORT_DSQLBK_TEST_PARAMS_JSON") == "" {
		t.Skip("DSQL backend tests are disabled. Enable them by setting the TELEPORT_DSQLBK_TEST_PARAMS_JSON variable.")
	}

	var cfg dsqlbk.Config
	require.NoError(t, json.Unmarshal([]byte(os.Getenv("TELEPORT_DSQLBK_TEST_PARAMS_JSON")), &cfg))
	require.NoError(t, cfg.CheckAndSetDefaults())
	if time.Duration(cfg.ExpiryInterval) > 500*time.Millisecond {
		t.Logf(
			"The expiry_interval set by TELEPORT_DSQLBK_TEST_PARAMS_JSON is %v which is longer than the recommended minimum of 500ms, consider setting it to 500ms or less if you encounter problems",
			time.Duration(cfg.ExpiryInterval),
		)
	}
}

func TestBackendComplianceSuite(t *testing.T) {
	checkTestParams(t)
	backendtest.RunBackendComplianceSuite(t, newTestBackend)
}

func TestAtomicWriteComplianceSuite(t *testing.T) {
	checkTestParams(t)
	backendtest.RunAtomicWriteComplianceSuite(t, newTestBackend)
}

func TestBackendComplianceSuiteWithAtomicWriteShim(t *testing.T) {
	checkTestParams(t)
	backendtest.RunBackendComplianceSuiteWithAtomicWriteShim(t, newTestBackend)
}

func newTestBackend(options ...backendtest.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
	testCfg, err := backendtest.ApplyOptions(options)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if testCfg.MirrorMode {
		return nil, nil, backendtest.ErrMirrorNotSupported
	}

	if testCfg.ConcurrentBackend != nil {
		return nil, nil, backendtest.ErrConcurrentAccessNotSupported
	}

	var config dsqlbk.Config
	if err := json.Unmarshal([]byte(os.Getenv("TELEPORT_DSQLBK_TEST_PARAMS_JSON")), &config); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	uut, err := dsqlbk.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return uut, backendtest.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
}
