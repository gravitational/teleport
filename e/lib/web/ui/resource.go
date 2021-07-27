package ui

import (
	"github.com/gravitational/teleport/api/v7/types"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"
)

// NewOIDCConnectors creates resource item for each given OIDC connector.
func NewOIDCConnectors(connectors []types.OIDCConnector) ([]ui.ResourceItem, error) {
	items := make([]ui.ResourceItem, 0, len(connectors))
	for _, oidc := range connectors {
		item, err := ui.NewResourceItem(oidc)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items = append(items, *item)
	}

	return items, nil
}

// NewSAMLConnectors creates resource item for each given SAML connector.
func NewSAMLConnectors(connectors []types.SAMLConnector) ([]ui.ResourceItem, error) {
	items := make([]ui.ResourceItem, 0, len(connectors))
	for _, saml := range connectors {
		item, err := ui.NewResourceItem(saml)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items = append(items, *item)
	}

	return items, nil
}
