package keystore

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
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
		callbacks     []error
	}{
		{
			desc:      "test success threshold",
			errs:      []error{nil, nil, nil},
			callbacks: []error{nil},
		},
		{
			desc:      "test failure threshold",
			errs:      []error{testErr, testErr, testErr, nil, nil, nil},
			callbacks: []error{testErr, nil},
		},
		{
			desc:      "test success threshold restart",
			errs:      []error{nil, nil, testErr, nil, nil, testErr, nil, nil, nil},
			callbacks: []error{nil},
		},
		{
			desc: "test non-unhealthy errors hit success threshold",
			errs: []error{
				&kmstypes.NotFoundException{},
				&kmstypes.NotFoundException{},
				&kmstypes.NotFoundException{},
			},
			callbacks: []error{nil},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			callbacks := make([]error, 0, len(tc.callbacks))
			h := passiveHealthChecker{
				callback: func(err error) {
					callbacks = append(callbacks, err)
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
				time.Sleep(time.Millisecond * 100)
			}
			require.Equal(t, len(tc.errs), probes)
			require.Equal(t, tc.callbacks, callbacks)
		})
	}
}
