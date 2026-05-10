package common

import (
	"context"
	"io"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"
)

func retryWithRelogin(ctx context.Context, tc *client.TeleportClient, fn func() error, opts ...client.RetryWithReloginOption) error {
	if isNonInteractiveWriter(tc.Stdout) {
		opts = append(opts, client.WithLoginStdout(tc.Stderr))
	}

	return client.RetryWithRelogin(ctx, tc, fn, opts...)
}

func isNonInteractiveWriter(w io.Writer) bool {
	return !utils.IsTerminal(w)
}
