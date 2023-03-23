package plugins

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/auth/oauth"
)

// ExchangerSet contains exchangers for different types of plugins
type ExchangerSet struct {
	Slack oauth.Exchanger
}

// GetExchanger returns an exchanger depending on plugin type
func (s *ExchangerSet) GetExchanger(plugin types.Plugin) (oauth.Exchanger, error) {
	v1, ok := plugin.(*types.PluginV1)
	if !ok {
		return nil, trace.BadParameter("unsupported plugin type %T, expected %T", plugin, v1)
	}

	switch v1.Spec.Settings.(type) {
	case *types.PluginSpecV1_SlackAccessPlugin:
		if s.Slack == nil {
			return nil, trace.NotFound("no exchanger is configured for Slack plugin")
		}
		return s.Slack, nil
	default:
		return nil, trace.BadParameter("unknown plugin type")
	}
}
