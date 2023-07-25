/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ui

import (
	"fmt"

	yaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// ResourceItem is UI representation of a resource (roles, trusted clusters, auth connectors).
type ResourceItem struct {
	// ID is a resource ID which is a composed value based on kind and name.
	// It is a composed value because while a resource name is unique to that resource,
	// the name can be the same for different resource type.
	ID string `json:"id"`
	// Kind is a resource kind.
	Kind string `json:"kind"`
	// Name is a resource name.
	Name string `json:"name"`
	// Content is resource yaml content.
	Content string `json:"content"`
}

// NewResourceItem creates UI objects for a resource.
func NewResourceItem(resource types.Resource) (*ResourceItem, error) {
	data, err := yaml.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kind := resource.GetKind()
	name := resource.GetName()

	return &ResourceItem{
		ID:      fmt.Sprintf("%v:%v", kind, name),
		Kind:    kind,
		Name:    name,
		Content: string(data[:]),
	}, nil

}

// NewRoles creates resource item for each role.
func NewRoles(roles []types.Role) ([]ResourceItem, error) {
	items := make([]ResourceItem, 0, len(roles))
	for _, role := range roles {
		// filter out system roles from web UI
		if types.IsSystemResource(role) {
			continue
		}

		item, err := NewResourceItem(role)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items = append(items, *item)
	}

	return items, nil
}

// NewGithubConnectors creates resource item for each github connector.
func NewGithubConnectors(connectors []types.GithubConnector) ([]ResourceItem, error) {
	items := make([]ResourceItem, 0, len(connectors))
	for _, connector := range connectors {
		item, err := NewResourceItem(connector)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items = append(items, *item)
	}

	return items, nil
}

// NewTrustedClusters creates resource item for each cluster.
func NewTrustedClusters(clusters []types.TrustedCluster) ([]ResourceItem, error) {
	items := make([]ResourceItem, 0, len(clusters))
	for _, cluster := range clusters {
		item, err := NewResourceItem(cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		items = append(items, *item)
	}

	return items, nil
}
