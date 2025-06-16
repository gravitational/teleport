package keystore

import (
	"context"
	"crypto"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

const (
	failureThreshold   = 3
	successThreshold   = 3
	healthCheckTimeout = time.Second * 10
)

var retryInterval = time.Second * 10

// passiveHealthChecker depends on client interactions with the keystore
// to trigger failures. Once triggered the probe function will be retried
// until it passes the success threshold.
type passiveHealthChecker struct {
	callback func(error)
	busy     atomic.Bool
	log      *slog.Logger
	clock    clockwork.Clock
}

type probeFunc func(context.Context) error

// tryProbe will call the probeFunc until the number of consecutive successful
// calls passes the successThreshold. This is a noop if a previous probe is still\
// running.
func (h *passiveHealthChecker) tryProbe(ctx context.Context, probe probeFunc) {
	if swapped := h.busy.CompareAndSwap(false, true); !swapped {
		return
	}
	go h.probeUntilHealthy(ctx, probe)
}

func (h *passiveHealthChecker) probeUntilHealthy(ctx context.Context, probe probeFunc) {
	var oks, fails uint
	start := h.clock.Now()
	timer := h.clock.NewTimer(retryInterval)
	defer h.busy.Store(false)

	h.log.DebugContext(ctx, "Starting passive health check probe")
	for {
		h.log.Info("Trying passive health check probe", "duration", h.clock.Since(start), "fails", fails, "oks", oks)
		timer.Reset(retryInterval)
		timeoutCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
		err := probe(timeoutCtx)
		cancel()

		if err != nil {
			fails += 1
			oks = 0
			h.log.DebugContext(ctx, "Detected error from passive health check probe", "err", err, "fails", fails, "oks", oks)
			if fails >= uint(failureThreshold) {
				h.log.DebugContext(ctx, "Passive health check failure threshold exceeded", "fails", fails, "oks", oks)
				h.callback(err)
			}
			continue
		}
		oks += 1
		if oks >= uint(successThreshold) {
			h.log.DebugContext(ctx, "Passive health check success threshold exceeded", "fails", fails, "oks", oks)
			h.callback(nil)
			return
		}

		h.log.DebugContext(ctx, "Passive health check succeeded", "fails", fails, "oks", oks)
		select {
		case <-timer.Chan():
		case <-ctx.Done():
		}
	}
}

// healthSigner wraps a signer with a callback to report failed sign requests.
type healthSigner struct {
	crypto.Signer
	health *passiveHealthChecker
}

// Sign wraps a [crypto.Signer] with a passive health check probe that gets called
// when an error occurs.
func (s *healthSigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	sig, err := s.Signer.Sign(rand, digest, opts)
	if err != nil {
		s.health.tryProbe(context.Background(), func(ctx context.Context) error {
			_, err := s.Signer.Sign(rand, digest, opts)
			return trace.Wrap(err)
		})
		return nil, trace.Wrap(err)
	}
	return sig, nil
}
