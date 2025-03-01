package launcher

import (
	"context"
	"github.com/gravitational/teleport/api/client"
	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/trace"
)

type Config interface {
	Services() []Service
	GetTeleportClient(ctx context.Context) (teleport.Client, error)
	PluginName() string
}

type Service interface {
	Run(ctx context.Context, clt teleport.Client, name string /* TODO add status sink */) error
	CheckHealth(ctx context.Context) error
}

// TODO: merge with integrations/lib/config.go

// BaseFileConfig is a configuration you can embed to read the standard teleport fields in your
// plugin file configs.
type BaseFileConfig struct {
	Teleport lib.TeleportConfig `toml:"teleport"`
	Log      logger.Config      `toml:"log"`
}

// GetTeleportClient returns a Teleport plugin client for the given config.
func (c BaseFileConfig) GetTeleportClient(ctx context.Context) (teleport.Client, error) {
	clt, err := c.Teleport.NewClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return wrapAPIClient(clt), nil
}

// TODO: check if we can get rid of this?

// wrapAPIClient will wrap the API client such that it conforms to the Teleport plugin client interface.
func wrapAPIClient(clt *client.Client) teleport.Client {
	return &wrappedClient{
		Client: clt,
	}
}

type wrappedClient struct {
	*client.Client
}

func (w *wrappedClient) ListAccessLists(ctx context.Context, pageSize int, pageToken string) ([]*accesslist.AccessList, string, error) {
	return w.Client.AccessListClient().ListAccessLists(ctx, pageSize, pageToken)
}

// ListAccessMonitoringRulesWithFilter lists current access monitoring rules.
func (w *wrappedClient) ListAccessMonitoringRulesWithFilter(ctx context.Context, pageSize int, pageToken string, subjects []string, notificationName string) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error) {
	return w.Client.AccessMonitoringRulesClient().ListAccessMonitoringRulesWithFilter(ctx, pageSize, pageToken, subjects, notificationName)
}
