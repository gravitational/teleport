package keystore

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestPassiveHealthCheck(t *testing.T) {
	clock := clockwork.NewFakeClock()
	testErr := errors.New("test")
	for _, tc := range []struct {
		desc          string
		retryInterval time.Duration
		errs          []error
		callbacks     int
	}{
		{
			desc:      "test success threshold",
			errs:      []error{nil, nil, nil},
			callbacks: 1,
		},
		{
			desc: "test failure threshold",

			errs:      []error{testErr, testErr, testErr, nil, nil, nil},
			callbacks: 2,
		},
		{
			desc:      "test success threshold restart",
			errs:      []error{nil, nil, testErr, nil, nil, testErr, nil, nil, nil},
			callbacks: 1,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			callbacks := 0
			h := passiveHealthChecker{
				callback: func(err error) {
					callbacks += 1
				},
				log:   slog.Default(),
				clock: clock,
			}

			ctx := context.Background()
			probes := 0
			retryInterval = tc.retryInterval
			h.tryProbe(ctx, func(ctx context.Context) error {
				err := tc.errs[probes]
				probes++
				return err
			})
			for h.busy.Load() {
			}
			require.Equal(t, len(tc.errs), probes)
			require.Equal(t, tc.callbacks, callbacks)
		})
	}
}
