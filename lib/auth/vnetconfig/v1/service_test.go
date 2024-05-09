package vnetconfig

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	header "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/gen/proto/go/teleport/vnet/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestVnetConfigService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	backend, err := memory.New(memory.Config{})
	require.NoError(t, err)

	storage, err := local.NewVnetConfigService(backend)
	require.NoError(t, err)

	checker := fakeChecker{
		allowedVerbs: []string{types.VerbRead, types.VerbUpdate, types.VerbCreate, types.VerbDelete},
	}

	authorizer := authz.AuthorizerFunc(func(ctx context.Context) (*authz.Context, error) {
		user, err := types.NewUser("alice")
		if err != nil {
			return nil, err
		}
		return &authz.Context{
			User:    user,
			Checker: checker,
		}, nil
	})

	service := NewVnetConfigService(storage, authorizer)

	vnetConfig := &vnet.VnetConfig{
		Kind:    types.KindVnetConfig,
		Version: types.V1,
		Metadata: &header.Metadata{
			Name: "vnet-config",
		},
		Spec: &vnet.VnetConfigSpec{
			CidrRange: "100.64.0.0/10",
			CustomDnsZones: []*vnet.CustomDNSZone{
				{Suffix: "example.com"},
				{Suffix: "test.example.com"},
			},
		},
	}

	createdVnetConfig, err := service.CreateVnetConfig(ctx, &vnet.CreateVnetConfigRequest{VnetConfig: vnetConfig})
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(vnetConfig, createdVnetConfig,
		cmpopts.IgnoreFields(header.Metadata{}, "Revision"),
		cmpopts.IgnoreUnexported(vnet.VnetConfig{}, vnet.VnetConfigSpec{}, vnet.CustomDNSZone{}, header.Metadata{})))
}

type fakeChecker struct {
	allowedVerbs []string
	services.AccessChecker
}

func (f fakeChecker) CheckAccessToRule(_ services.RuleContext, _ string, resource string, verb string) error {
	if resource == types.KindVnetConfig {
		for _, allowedVerb := range f.allowedVerbs {
			if allowedVerb == verb {
				return nil
			}
		}
	}

	return trace.AccessDenied("access denied to rule=%v/verb=%v", resource, verb)
}
