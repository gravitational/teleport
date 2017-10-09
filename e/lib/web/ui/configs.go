package ui

import (
	"fmt"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// ConfigCollection is a collection of ConfigItems
type ConfigCollection struct {
	// Items is a slice of ConfigItems
	Items []ConfigItem `json:"items"`
}

// ConfigItem is UI representation of the resource
type ConfigItem struct {
	// ID is a resource ID which is a composed value based on kind and name
	ID string `json:"id"`
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Name is a resource name
	Name string `json:"name"`
	// DisplayName is a resource display name
	DisplayName string `json:"displayName"`
	// Content is resource yaml content
	Content string `json:"content"`
}

// ConvertRoles creates UI objects for Roles
func ConvertRoles(roles []services.Role) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, role := range roles {
		item, err := NewConfigItem(services.KindRole, role.GetName(), role)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertTrustedClusters creates UI objects for Cluster
func ConvertTrustedClusters(clusters []services.TrustedCluster) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, cluster := range clusters {
		item, err := NewConfigItem(services.KindTrustedCluster, cluster.GetName(), cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertOIDCConnectors creates UI objects for OIDC connectors
func ConvertOIDCConnectors(connectors []services.OIDCConnector) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, oidc := range connectors {
		item, err := NewConfigItem(services.KindOIDCConnector, oidc.GetName(), oidc)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertSAMLConnectors creates UI objects for SAML connectors
func ConvertSAMLConnectors(connectors []services.SAMLConnector) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, saml := range connectors {
		item, err := NewConfigItem(services.KindSAMLConnector, saml.GetName(), saml)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// NewConfigItem creates UI objects for resource
func NewConfigItem(kind string, name string, resource interface{}) (*ConfigItem, error) {
	data, err := yaml.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ConfigItem{
		ID:      fmt.Sprintf("%v:%v", kind, name),
		Kind:    kind,
		Name:    name,
		Content: string(data[:]),
	}, nil

}
