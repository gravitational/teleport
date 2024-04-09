package integration

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// RunAndWaitReady is a helper to start an app implementing AppI and wait for
// it to become ready.
// This is used to start plugins.
func RunAndWaitReady(t *testing.T, app AppI) {
	appCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go func() {
		ctx := appCtx
		if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			assert.ErrorContains(t, err, context.Canceled.Error(), "if a non-nil error is returned, it should be canceled context")
		}
	}()

	t.Cleanup(func() {
		err := app.Shutdown(appCtx)
		assert.NoError(t, err)
		err = app.Err()
		if err != nil {
			assert.ErrorContains(t, err, context.Canceled.Error(), "if a non-nil error is returned, it should be canceled context")
		}
	})

	waitCtx, cancel := context.WithTimeout(appCtx, 20*time.Second)
	defer cancel()

	ok, err := app.WaitReady(waitCtx)
	require.NoError(t, err)
	require.True(t, ok)
}
