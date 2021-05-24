package api

import (
	"context"
	"time"

	"github.com/gravitational/teleport/lib/term/proto"

	"github.com/gravitational/trace/trail"
	log "github.com/sirupsen/logrus"
)

// Handler implements multiple API services
type Handler struct {
}

// Subscribe returns a stream of tick events
func (t *Handler) Subscribe(req *proto.TickRequest, stream proto.TickService_SubscribeServer) error {
	log.Infof("Subscribe")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case tick := <-ticker.C:
			if err := stream.Send(&proto.Tick{Time: tick.UnixNano()}); err != nil {
				return trail.ToGRPC(err)
			}
		}
	}
}

// Now return current time
func (t *Handler) Now(ctx context.Context, _ *proto.TickRequest) (*proto.Tick, error) {
	log.Infof("Now")
	return &proto.Tick{
		Time: time.Now().UTC().UnixNano(),
	}, nil
}
