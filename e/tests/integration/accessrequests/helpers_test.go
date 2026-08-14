package accessrequests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/tests/common"
)

func mustCreateNode(ctx context.Context, t *testing.T, dst common.NodeUpserter, name, hostname string, options ...common.NodeOption) *types.ServerV2 {
	t.Helper()
	n, err := common.CreateNode(ctx, dst, name, hostname, options...)
	require.NoError(t, err)
	return n
}

// resourceList defines the wire format for an arbitrary list of resources.
type resourceList[T any] struct {
	// Items is a list of resources retrieved.
	Items []T `json:"items"`
	// StartKey is the position to resume search events.
	StartKey string `json:"startKey"`
	// TotalCount is the total count of resources available
	// after filter.
	TotalCount int `json:"totalCount"`
}
