package identitycenter

import (
	"context"
)

// resourceEventLoop handles events from the resource monitor. Does nothing but
// drain the event channel at present, but will be extended to process events in
// a later PR
func (svc *Service) resourceEventLoop(ctx context.Context) error {
	for {
		select {
		case _, ok := <-svc.principalEventCh:
			if !ok {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}
