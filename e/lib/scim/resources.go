package scim

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/elimity-com/scim/schema"
	"github.com/gravitational/trace"
	"github.com/scim2/filter-parser/v2"

	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
)

// resourceHandler is an abstraction for providing CRUD over different resource
// types.
type resourceHandler interface {
	create(context.Context, providerShim, *scimpb.Resource) (*scimpb.Resource, error)
	list(context.Context, providerShim, filter.Expression, *scimpb.Page) (*scimpb.ResourceList, error)
	get(context.Context, providerShim, string) (*scimpb.Resource, error)
	update(context.Context, providerShim, *scimpb.Resource) (*scimpb.Resource, error)
	delete(context.Context, providerShim, string) error
}

// resourceTypeHandler acts as the glue between the top-level SCIM service and the
// ability to read and write a given concrete resource (e.g. users, groups),
// which is implemented by a an underling `resourceHandler` implementation. All
// of the behavior common across resources belongs here, while resource-specific
// behavior belongs in the underlying `resourceHandler`
type resourceTypeHandler struct {
	name     string
	endpoint string
	schema   schema.Schema
	handler  resourceHandler
	logger   *slog.Logger
}

// getResource fetches a single resource from the underlying `resourceHandler`
// implementation, validates it and forwards it back to the Service as a
// generic Resource.
func (t *resourceTypeHandler) getResource(ctx context.Context, shim providerShim, target *scimpb.RequestTarget) (*scimpb.Resource, error) {
	resource, err := t.handler.get(ctx, shim, target.ResourceId)
	if err != nil {
		return nil, trace.Wrap(err, "get %s", target.ResourceId)
	}

	t.ensureMetadata(resource)

	return resource, nil
}

// listResource fetches a collection of resources from the underlying
// `resourceHandler` implementation, validates them  and forwards it back to
// the Service for transmission to the client.
func (t *resourceTypeHandler) listResources(ctx context.Context, shim providerShim, filterText string, requestedPage *scimpb.Page) (*scimpb.ResourceList, error) {
	filter, err := ParseFilter(filterText)
	if err != nil {
		return nil, trace.Wrap(err, "parsing and validating filter")
	}

	if requestedPage == nil {
		requestedPage = &scimpb.Page{StartIndex: 1, Count: 100}
	}

	outputPage, err := t.handler.list(ctx, shim, filter, requestedPage)
	if err != nil {
		return nil, trace.Wrap(err, "listing resources")
	}

	for _, r := range outputPage.Resources {
		t.ensureMetadata(r)
	}

	return outputPage, nil
}

// createResource creates a new resource from the supplied resource description
func (t *resourceTypeHandler) createResource(ctx context.Context, shim providerShim, res *scimpb.Resource) (*scimpb.Resource, error) {
	newRes, err := t.handler.create(ctx, shim, res)
	if err != nil {
		return nil, trace.Wrap(err, "failed creating resource")
	}

	t.ensureMetadata(newRes)

	return newRes, nil
}

func (t *resourceTypeHandler) updateResource(ctx context.Context, shim providerShim, res *scimpb.Resource) (*scimpb.Resource, error) {

	outputResource, err := t.handler.update(ctx, shim, res)
	if err != nil {
		return nil, trace.Wrap(err, "failed updating resource")
	}

	t.ensureMetadata(outputResource)

	return outputResource, nil
}

func (t *resourceTypeHandler) deleteResource(ctx context.Context, shim providerShim, resourceID string) error {
	return trace.Wrap(t.handler.delete(ctx, shim, resourceID))
}

func (t *resourceTypeHandler) ensureMetadata(res *scimpb.Resource) {
	if len(res.Schemas) == 0 {
		res.Schemas = append(res.Schemas, t.schema.ID)
	}

	if res.Meta.Location == "" {
		res.Meta.Location = fmt.Sprintf("%s/%s", t.endpoint, res.Id)
	}
}
