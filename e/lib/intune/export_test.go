package intune

import (
	"context"
	"time"

	"github.com/gravitational/teleport/e/lib/mdm"
)

// RunFullSync lets tests synchronously trigger a full sync.
func (s *Service) RunFullSync(ctx context.Context) (nextDeviceLastSyncDateTime time.Time, err error) {
	return s.runWithSpec(ctx, runSpec{mode: mdm.SyncModeFull})
}
