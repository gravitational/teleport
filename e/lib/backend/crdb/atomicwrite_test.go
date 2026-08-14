package crdb

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// newAtomicWriteTestBackend builds a backend suitable for the atomic write test suite. Once all backends implement AtomicWrite,
// it will be integrated into the main backend interface and we can get rid of this separate helper.
func newAtomicWriteTestBackend(options ...test.ConstructionOption) (backend.AtomicWriterBackend, clocki.FakeClock, error) {
	testCfg, err := test.ApplyOptions(options)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if testCfg.MirrorMode {
		return nil, nil, test.ErrMirrorNotSupported
	}

	if testCfg.ConcurrentBackend != nil {
		return nil, nil, test.ErrConcurrentAccessNotSupported
	}

	var params backend.Params
	if err := json.Unmarshal([]byte(os.Getenv("TELEPORT_CRDB_TEST_PARAMS_JSON")), &params); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	uut, err := NewFromParams(context.Background(), params)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return uut, test.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	getParamString(t) // skips if params not set

	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackend)
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	getParamString(t) // skips if params not set

	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackend)
}
