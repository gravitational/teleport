package inventory

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/trace"
)

func (c *Controller) handleClockRequest(handle *upstreamHandle, req clockRequest) error {
	clock := proto.DownstreamInventoryClockRequest{
		ID: req.id,
	}
	if err := handle.Send(c.closeContext, clock); err != nil {
		req.rspC <- clockResponse{
			err: err,
		}
		return trace.Wrap(err)
	}
	handle.clockRequests[req.id] = pendingClock{
		start: c.clock.Now(),
		rspC:  req.rspC,
	}
	return nil
}

func (c *Controller) handleClockResponse(handle *upstreamHandle, msg proto.UpstreamInventoryClockResponse) {
	pending, ok := handle.clockRequests[msg.ID]
	if !ok {
		slog.WarnContext(context.Background(), "Unexpected upstream clock from server",
			"server", handle.Hello().ServerID, "id", msg.ID)
		return
	}
	pending.rspC <- clockResponse{
		reqD:      c.clock.Since(pending.start),
		clockDiff: c.clock.Since(msg.LocalTime),
	}
	delete(handle.clockRequests, msg.ID)
}
