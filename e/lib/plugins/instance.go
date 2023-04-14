package plugins

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
	"github.com/gravitational/teleport/integrations/access/common/auth/storage"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
)

// instanceDependencies is a container for dependencies of a plugin instance.
type instanceDependencies struct {
	authorizer oauth.Authorizer
	client     teleport.Client
	store      storage.Store
	statusSink common.StatusSink

	log *logrus.Entry
}

// instanceFactory takes plugin spec and its dependencies,
// and returns a function that runs the plugin instance.
type instanceFactory func(ctx context.Context, plugin *types.PluginV1, deps instanceDependencies) (func() error, error)

// instance represents a single plugin instance.
type instance struct {
	// cancel is a function that closes the instance's context
	cancel func()
	// spec is the spec that the instance is currently configured with
	spec *types.PluginSpecV1
}
