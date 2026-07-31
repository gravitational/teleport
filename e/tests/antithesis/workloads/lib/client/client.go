// Package client provides common client constructors for Antithesis workload
// tests.
package client

import (
	"context"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
)

// NewAPIClient creates a Teleport API client for a workload identity.
func NewAPIClient(ctx context.Context, identity string) (*apiclient.Client, error) {
	return apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{testenv.ProxyAddr()},
		Credentials: []apiclient.Credentials{apiclient.LoadIdentityFile(testenv.IdentityPath(identity))},
	})
}
