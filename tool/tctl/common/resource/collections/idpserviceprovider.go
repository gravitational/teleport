package collections

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"io"
)

//nolint:revive // Because we want this to be IdP.
type samlIdPServiceProviderCollection struct {
	serviceProviders []types.SAMLIdPServiceProvider
}

func NewSAMLIdPServiceProviderCollection(serviceProviders []types.SAMLIdPServiceProvider) ResourceCollection {
	return &samlIdPServiceProviderCollection{serviceProviders: serviceProviders}
}

func (c *samlIdPServiceProviderCollection) Resources() []types.Resource {
	r := make([]types.Resource, len(c.serviceProviders))
	for i, resource := range c.serviceProviders {
		r[i] = resource
	}
	return r
}

func (c *samlIdPServiceProviderCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, serviceProvider := range c.serviceProviders {
		t.AddRow([]string{serviceProvider.GetName()})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}
