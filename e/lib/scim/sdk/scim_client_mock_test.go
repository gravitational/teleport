package scimsdk

import (
	"context"
	"testing"
)

func TestSCIMClientMock(t *testing.T) {
	ctx := context.Background()

	cli := NewSCIMClientMock()
	testSCIMIntegration(t, ctx, cli)
}
