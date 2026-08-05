// Package client provides common client constructors for Antithesis workload
// tests.
package client

import (
	"context"
	"iter"
	"time"

	apiclient "github.com/gravitational/teleport/api/client"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/tests/antithesis/workloads/lib/testenv"
)

// NewAPIClient creates a Teleport API client for a workload identity.
func NewAPIClient(ctx context.Context, identity string) (*apiclient.Client, error) {
	return apiclient.New(ctx, apiclient.Config{
		Addrs:       []string{testenv.ProxyAddr()},
		Credentials: []apiclient.Credentials{apiclient.LoadIdentityFile(testenv.IdentityPath(identity))},
	})
}

// RangeAllAuditEventsByType is a helper around [apiclient.Client.SearchEvents] to search all events between start and end matching given eventTypes
func RangeAllAuditEventsByType(ctx context.Context, clt *apiclient.Client, start, end time.Time, eventTypes ...string) iter.Seq2[apievents.AuditEvent, error] {
	pageFn := func(ctx context.Context, limit int, startKey string) ([]apievents.AuditEvent, string, error) {
		return clt.SearchEvents(
			ctx, start, end, apidefaults.Namespace,
			eventTypes,
			limit, types.EventOrderAscending, startKey, "")
	}

	return clientutils.Resources(ctx, pageFn)
}
