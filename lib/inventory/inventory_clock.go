package inventory

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
)

type pendingClock struct {
	start time.Time
	rspC  chan clockResponse
}

type clockRequest struct {
	id   uint64
	rspC chan clockResponse
}

type clockResponse struct {
	clockDiff time.Duration
	reqD      time.Duration
	err       error
}

func (h *upstreamHandle) TimeReconciliation(ctx context.Context, id uint64) (d time.Duration, err error) {
	rspC := make(chan clockResponse, 1)
	select {
	case h.clockC <- clockRequest{rspC: rspC, id: id}:
	case <-h.Done():
		return 0, trace.Errorf("failed to send downstream clock (stream closed)")
	case <-ctx.Done():
		return 0, trace.Errorf("failed to send downstream clock: %v", ctx.Err())
	}

	select {
	case rsp := <-rspC:
		if rsp.clockDiff > 0 {
			return rsp.clockDiff - rsp.reqD, nil
		} else {
			return rsp.clockDiff + rsp.reqD, nil
		}
	case <-h.Done():
		return 0, trace.Errorf("failed to recv upstream clock response (stream closed)")
	case <-ctx.Done():
		return 0, trace.Errorf("failed to recv upstream clock request: %v", ctx.Err())
	}
}

func (h *downstreamHandle) handleClock(sender DownstreamSender, msg proto.DownstreamInventoryClockRequest) {
	h.mu.Lock()
	defer h.mu.Unlock()
	go func() {
		err := sender.Send(context.Background(), proto.UpstreamInventoryClockResponse{
			ID:        msg.ID,
			LocalTime: h.clock.Now().UTC(),
		})
		if err != nil {
			slog.ErrorContext(context.Background(), "Got clock with no handlers registered.", "id", msg.ID)
		}
	}()
}
