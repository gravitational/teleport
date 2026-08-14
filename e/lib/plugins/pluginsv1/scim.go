package pluginsv1

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

type scimPluginHandler struct {
	defaultHandler
}

type getConnectorFunc func(context.Context, string, bool) error

func wrapGetConnector[T any](get func(context.Context, string, bool) (T, error)) getConnectorFunc {
	return func(ctx context.Context, name string, withSecrets bool) error {
		_, err := get(ctx, name, withSecrets)
		return trace.Wrap(err)
	}
}

func (s scimPluginHandler) validatePlugin(ctx context.Context, input pluginValidationInput, auth *auth.Server) error {
	scim := input.plugin.Spec.GetScim()
	if scim == nil {
		return trace.BadParameter("missing SCIM settings")
	}

	if err := s.validateConnector(ctx, input.plugin, auth); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (scimPluginHandler) validateConnector(ctx context.Context, plugin *types.PluginV1, auth *auth.Server) error {
	scim := plugin.Spec.GetScim()
	if scim == nil {
		return trace.BadParameter("missing SCIM settings")
	}

	var connectorGetHandler = map[string]getConnectorFunc{
		types.KindSAML: wrapGetConnector(auth.GetSAMLConnector),
		types.KindOIDC: wrapGetConnector(auth.GetOIDCConnector),
	}

	if isConnectorTypeMissing(scim) {
		// For better UX experience, if the SCIM plugin settings have a connector
		// name but no type, we try to deduce the type based on the connector name state configured
		// in the backend.
		if err := deduceConnectorType(ctx, scim, connectorGetHandler); err != nil {
			return trace.Wrap(err)
		}
	}

	connInfo, err := GetSCIMPluginConnectorInfo(scim)
	if err != nil {
		return trace.Wrap(err)
	}

	// Ensure that connector that SCIM plugin is referencing exists.
	connGetter, ok := connectorGetHandler[connInfo.Type]
	if !ok {
		validKinds := slices.Collect(maps.Keys(connectorGetHandler))
		return trace.BadParameter("unsupported connector type %q: expected one of %q", connInfo.Type, validKinds)
	}

	if err := connGetter(ctx, connInfo.Name, false); err != nil {
		if trace.IsNotFound(err) {
			return trace.BadParameter("SCIM references %s connector %q, but it does not exist", connInfo.Type, connInfo.Name)
		}
		return trace.Wrap(err)
	}

	return nil

}

func deduceConnectorType(ctx context.Context, scim *types.PluginSCIMSettings, getters map[string]getConnectorFunc) error {
	if scim.ConnectorInfo == nil {
		return trace.BadParameter("connector_info must be set when deducing connector type")
	}
	var deducedType string
	for kind, get := range getters {
		if err := get(ctx, scim.ConnectorInfo.Name, false); err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			slog.WarnContext(ctx, "Failed to get connector.",
				"connector", scim.ConnectorInfo.Name,
				"type", kind,
				"error", err,
			)
			continue
		}
		if deducedType != "" {
			return trace.BadParameter("multiple connectors found with name %q with different types; please specify connector type", scim.ConnectorInfo.Name)
		}
		deducedType = kind
	}
	if deducedType == "" {
		return trace.BadParameter("no connector found with name %q", scim.ConnectorInfo.Name)
	}
	scim.ConnectorInfo.Type = deducedType
	return nil
}

func GetSCIMPluginConnectorInfo(scim *types.PluginSCIMSettings) (*types.PluginSCIMSettings_ConnectorInfo, error) {
	if scim == nil {
		return nil, trace.BadParameter("SCIM plugin settings cannot be nil")
	}

	// Either saml_connector_name OR connector_info must be provided, but not both.
	if scim.SamlConnectorName != "" && scim.ConnectorInfo != nil {
		return nil, trace.BadParameter("SCIM plugin must specify either saml_connector_name or connector_info, not both")
	}

	if scim.SamlConnectorName != "" {
		// For backward compatibility, if only saml_connector_name is provided,
		// The connector_info is constructed with the SAML type.
		return &types.PluginSCIMSettings_ConnectorInfo{
			Type: types.KindSAML,
			Name: scim.SamlConnectorName,
		}, nil
	}

	if scim.ConnectorInfo == nil {
		return nil, trace.BadParameter("SCIM plugin must specify either saml_connector_name or connector_info")
	}

	if scim.ConnectorInfo.Name == "" {
		return nil, trace.BadParameter("connector_info.name must be set")
	}

	switch scim.ConnectorInfo.Type {
	case types.KindSAML, types.KindOIDC:
		return scim.ConnectorInfo, nil
	default:
		return nil, trace.BadParameter("unsupported connector type %q", scim.ConnectorInfo.Type)
	}
}

// isConnectorTypeMissing returns true if the SCIM settings have a connector
// name but no type, meaning we should try to deduce the type by name.
func isConnectorTypeMissing(scim *types.PluginSCIMSettings) bool {
	return scim != nil &&
		scim.ConnectorInfo != nil &&
		scim.ConnectorInfo.Type == "" &&
		scim.ConnectorInfo.Name != ""
}
