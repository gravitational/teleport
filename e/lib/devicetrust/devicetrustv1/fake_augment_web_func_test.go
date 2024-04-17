package devicetrustv1_test

import (
	"context"
	"errors"
	"sync"

	"github.com/gravitational/teleport/lib/auth"
)

var errFakeAugmentWebFuncFailed = errors.New("failed to augment web session certificates")

type fakeAugmentWebFunc struct {
	mu       sync.Mutex
	failNext map[string]bool // key is the sessionID
	numCalls map[string]int  // key is the sessionID
}

func (f *fakeAugmentWebFunc) setFailNext(sessionID string) {
	f.mu.Lock()
	if f.failNext == nil {
		f.failNext = make(map[string]bool)
	}
	f.failNext[sessionID] = true
	f.mu.Unlock()
}

func (f *fakeAugmentWebFunc) getNumCalls(sessionID string) int {
	f.mu.Lock()
	val := f.numCalls[sessionID]
	f.mu.Unlock()
	return val
}

// function runs the actual AugmentWebSessionCertificates function.
func (f *fakeAugmentWebFunc) function(_ context.Context, opts *auth.AugmentWebSessionCertificatesOpts) error {
	// Run a few basic checks.
	switch {
	case opts == nil:
		return errors.New("opts required")
	case opts.WebSessionID == "":
		return errors.New("opts.WebSessionID required")
	case opts.User == "":
		return errors.New("opts.User required")
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
