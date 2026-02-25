package reversetunnel

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnel/track"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
)

type Client struct {
	mu   sync.Mutex
	done bool
	wg   sync.WaitGroup
}

type authClient interface {
	GetClusterNetworkingConfig(context.Context) (apitypes.ClusterNetworkingConfig, error)
}

func (c *Client) Run(
	ctx context.Context,
	log *slog.Logger,
	clusterName string,
	clt authClient,
	resolver reversetunnelclient.Resolver,
) error {
	c.mu.Lock()
	if c.done {
		c.mu.Unlock()
		return trace.Errorf("already closed")
	}
	c.wg.Add(1)
	defer c.wg.Done()
	c.mu.Unlock()

	tracker, err := track.New(track.Config{ClusterName: clusterName})
	if err != nil {
		return trace.Wrap(err)
	}

	cnc, err := clt.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	var agentConnectionCount int
	if ppts := cnc.GetProxyPeeringTunnelStrategy(); ppts != nil {
		agentConnectionCount = int(ppts.AgentConnectionCount)
	}
	// TODO: update this periodically
	tracker.SetConnectionCount(agentConnectionCount)

	t := time.NewTicker(time.Second)
	defer t.Stop()

	for ctx.Err() == nil {
		if lease := tracker.TryAcquire(); lease != nil {
			log.InfoContext(ctx, "spawning new reversetunnel client connection")
			c.wg.Go(func() {
				err := c.run(lease, resolver)
				log.WarnContext(ctx, "reversetunnel client connection ended", "error", err)
			})
			continue
		}

		select {
		case <-ctx.Done():
		case <-t.C:
		}
	}

	return trace.Wrap(ctx.Err())
}

func (c *Client) run(lease *track.Lease, resolver reversetunnelclient.Resolver) error {
	panic("todo")
}

func (c *Client) Stop(ctx context.Context, log *slog.Logger) {
	c.mu.Lock()
	c.done = true
	c.mu.Unlock()
	c.wg.Wait()
}
