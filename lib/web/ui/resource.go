/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
	// Description is an optional resource description.
	Description string `json:"description,omitempty"`
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
	description := resource.GetMetadata().Description

	return &ResourceItem{
		ID:          fmt.Sprintf("%v:%v", kind, name),
		Kind:        kind,
		Name:        name,
		Description: description,
		Content:     string(data[:]),
	}, nil

}

// NewRoles creates resource item for each role.
func NewRoles(roles []types.Role) ([]ResourceItem, error) {
	items := make([]ResourceItem, 0, len(roles))
	for _, role := range roles {
		// filter out system roles from web UI
		// TODO(gzdunek): DELETE IN 17.0.0: We filter out the roles in the auth server.
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
