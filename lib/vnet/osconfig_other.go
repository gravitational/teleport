//go:build !darwin
// +build !darwin

package vnet

import (
	"context"
	"runtime"

	"github.com/gravitational/trace"
)

func configureOS(ctx context.Context, cfg *osConfig) error {
	return trace.NotImplemented("configureOS is not implemented on %s", runtime.GOOS)
}
