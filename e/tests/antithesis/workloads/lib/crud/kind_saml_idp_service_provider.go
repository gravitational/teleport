package crud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// SAMLIdPServiceProviderOpsForClient returns type-erased CRUD operations for SAML IdP service provider resources.
func SAMLIdPServiceProviderOpsForClient(client any) (KindResourceOps, error) {
	c, ok := client.(services.SAMLIdPServiceProviders)
	if !ok {
		return nil, trace.NotImplemented("CRUD operations for kind %q not implemented", types.KindSAMLIdPServiceProvider)
	}
	return SAMLIdPServiceProviderOps(c), nil
}

type samlIDPServiceProviderOps struct {
	clt services.SAMLIdPServiceProviders
}

type samlIDPServiceProviderKindOps struct {
	*samlIDPServiceProviderOps
	*resourceOpsLegacy[*samlIDPServiceProviderOps, types.SAMLIdPServiceProvider]
}

// SAMLIdPServiceProviderOps returns CRUD operations for SAML IdP service provider resources.
func SAMLIdPServiceProviderOps(client services.SAMLIdPServiceProviders) KindResourceOps {
	ops := &samlIDPServiceProviderOps{clt: client}
	return &samlIDPServiceProviderKindOps{
		samlIDPServiceProviderOps: ops,
		resourceOpsLegacy:         &resourceOpsLegacy[*samlIDPServiceProviderOps, types.SAMLIdPServiceProvider]{ops: ops},
	}
}

func (o *samlIDPServiceProviderOps) Kind() string {
	return types.KindSAMLIdPServiceProvider
}

func (o *samlIDPServiceProviderOps) NewResource(name string) (types.Resource153, error) {
	provider, err := types.NewSAMLIdPServiceProvider(types.Metadata{Name: name}, types.SAMLIdPServiceProviderSpecV1{
		EntityID: "https://example.com/" + name,
		ACSURL:   "https://example.com/saml/acs",
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.LegacyToResource153(provider), nil
}

func (o *samlIDPServiceProviderOps) Clone(resource types.SAMLIdPServiceProvider) types.SAMLIdPServiceProvider {
	return resource.Copy()
}

func (o *samlIDPServiceProviderOps) Create(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	provider, err := unwrapLegacy[types.SAMLIdPServiceProvider](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.CreateSAMLIdPServiceProvider(ctx, provider))
}

func (o *samlIDPServiceProviderOps) Get(ctx context.Context, name string) (types.SAMLIdPServiceProvider, error) {
	return o.clt.GetSAMLIdPServiceProvider(ctx, name)
}

func (o *samlIDPServiceProviderOps) List(ctx context.Context, pageSize int, pageToken string) ([]types.SAMLIdPServiceProvider, string, error) {
	providers, next, err := o.clt.ListSAMLIdPServiceProviders(ctx, pageSize, pageToken)
	return providers, next, trace.Wrap(err)
}

func (o *samlIDPServiceProviderOps) Update(ctx context.Context, resource types.Resource153) (types.Resource153, error) {
	provider, err := unwrapLegacy[types.SAMLIdPServiceProvider](resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.Wrap(o.clt.UpdateSAMLIdPServiceProvider(ctx, provider))
}

func (o *samlIDPServiceProviderOps) Delete(ctx context.Context, name string) error {
	return trace.Wrap(o.clt.DeleteSAMLIdPServiceProvider(ctx, name))
}

func (o *samlIDPServiceProviderOps) DeleteAll(ctx context.Context) error {
	return trace.Wrap(o.clt.DeleteAllSAMLIdPServiceProviders(ctx))
}

func (o *samlIDPServiceProviderOps) SupportsDeleteAll() bool {
	return true
}

func (o *samlIDPServiceProviderOps) Equal(a, b types.Resource153) bool {
	// TODO(okraport): add IsEqual support to types.SAMLIdPServiceProvider and enable this kind.
	panic("resource kind \"saml_idp_service_provider\" has no Equal comparison configured")
}
