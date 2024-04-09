package devicetrustv1_test

import (
	"context"
	"errors"
	"sync"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
)

// TODO(codingllama): Address fakeAugmentWebFunc "unused" nolints.

//nolint:unused // To be used by ConfirmDeviceWebAuthentication tests.
var errFakeAugmentWebFuncFailed = errors.New("failed to augment web session certificates")

//nolint:unused // To be used by ConfirmDeviceWebAuthentication tests.
type fakeAugmentWebFunc struct {
	mu       sync.Mutex
	failNext map[string]bool // key is the sessionID
	numCalls map[string]int  // key is the sessionID
}

//nolint:unused // To be used by ConfirmDeviceWebAuthentication tests.
func (f *fakeAugmentWebFunc) setFailNext(sessionID string) {
	f.mu.Lock()
	if f.failNext == nil {
		f.failNext = make(map[string]bool)
	}
	f.failNext[sessionID] = true
	f.mu.Unlock()
}

//nolint:unused // To be used by ConfirmDeviceWebAuthentication tests.
func (f *fakeAugmentWebFunc) getNumCalls(sessionID string) int {
	f.mu.Lock()
	val := f.numCalls[sessionID]
	f.mu.Unlock()
	return val
}

// function runs the actual AugmentWebSessionCertificates function.
//
//nolint:unused // To be used by ConfirmDeviceWebAuthentication tests.
func (f *fakeAugmentWebFunc) function(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentWebSessionCertificatesOpts) error {
	// Run a few basic checks.
	switch {
	case authCtx == nil:
		return errors.New("authCtx required")
	case opts == nil:
		return errors.New("opts required")
	case opts.WebSessionID == "":
		return errors.New("opts.WebSessionID required")
	case opts.DeviceExtensions == nil:
		return errors.New("opts.DeviceExtensions required")
	}
	sessionID := opts.WebSessionID

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.numCalls == nil {
		f.numCalls = make(map[string]int)
	}
	f.numCalls[sessionID]++

	if f.failNext[sessionID] {
		f.failNext[sessionID] = false
		return errFakeAugmentWebFuncFailed
	}

	return nil
}
