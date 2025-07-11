package pluginsv1

import (
	"context"
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

type scimPluginHandler struct {
	defaultHandler
}

func (scimPluginHandler) validatePlugin(ctx context.Context, plugin *types.PluginV1, auth *auth.Server) error {
	if plugin.Spec.GetScim() == nil {
		return errors.New("missing SCIM settings")
	}

	connectorName := plugin.Spec.GetScim().SamlConnectorName
	if _, err := auth.GetSAMLConnector(ctx, connectorName, false); err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("references to SAML connector %q that does not exist", connectorName)
		}
		return trace.Wrap(err)
	}
	return nil
}
