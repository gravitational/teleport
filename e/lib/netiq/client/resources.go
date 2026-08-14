package client

import (
	"context"

	"github.com/gravitational/trace"
)

// Resource represents a resource in the Identity Vault.
type Resource struct {
	// ID is the unique identifier of the resource.
	ID string `json:"id"`
	// Name is the name of the resource.
	Name string `json:"name"`
	// Description is the description of the resource.
	Description string `json:"description"`
	// Categories is the list of categories the resource belongs to.
	Categories []Category `json:"categories,omitempty"`
}

// Category represents a category in the Identity Vault.
type Category struct {
	// ID is the unique identifier of the category.
	ID string `json:"id"`
	// Name is the name of the category.
	Name string `json:"name"`
}

// ListResources returns a list of all resources in the Identity Vault.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	const rolesBase = "rest/catalog/resources/listV2"

	roles, err := listResponse(
		ctx,
		c,
		rolesBase,
		func(r listResourcesResponse) ([]Resource, error) {
			return r.Resources, nil
		},
		withQueryParams("q", "*"),
	)

	return roles, trace.Wrap(err)
}

type listResourcesResponse struct {
	NextIndex int        `json:"nextIndex"`
	ArraySize int        `json:"arraySize"`
	Resources []Resource `json:"resources"`
	TotalSize int        `json:"totalSize"`
	Total     int        `json:"total"`
}

func (n listResourcesResponse) GetNextIndex() int {
	return n.NextIndex
}
