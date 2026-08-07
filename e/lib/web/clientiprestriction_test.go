package web

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/types"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

// fakeCloudCIRClient stands in for the Cloud API behind the ClientIPRestriction
// gRPC service. It embeds the client interface so any RPC a test does not stub is
// nil and panics, which is louder than silently returning a zero value.
type fakeCloudCIRClient struct {
	cloudapi.TenantsServiceClient

	get    func(*cloudapi.GetClientIPRestrictionRequest) (*cloudapi.GetClientIPRestrictionResponse, error)
	update func(*cloudapi.UpdateClientIPRestrictionRequest) (*cloudapi.UpdateClientIPRestrictionResponse, error)
	upsert func(*cloudapi.UpsertClientIPRestrictionRequest) (*cloudapi.UpsertClientIPRestrictionResponse, error)
}

func (f *fakeCloudCIRClient) GetClientIPRestriction(_ context.Context, in *cloudapi.GetClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudapi.GetClientIPRestrictionResponse, error) {
	return f.get(in)
}

func (f *fakeCloudCIRClient) UpdateClientIPRestriction(_ context.Context, in *cloudapi.UpdateClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudapi.UpdateClientIPRestrictionResponse, error) {
	return f.update(in)
}

func (f *fakeCloudCIRClient) UpsertClientIPRestriction(_ context.Context, in *cloudapi.UpsertClientIPRestrictionRequest, _ ...grpc.CallOption) (*cloudapi.UpsertClientIPRestrictionResponse, error) {
	return f.upsert(in)
}

// newCIRTestEnv brings up a proxy whose auth server registers the
// ClientIPRestriction service (it is gated on the Cloud feature) and points that
// service at a fake Cloud API, so the real endpoints can be driven over HTTP.
func newCIRTestEnv(t *testing.T, cloudClient cloudapi.TenantsServiceClient) (*webSuite, *authWebPack, string) {
	t.Helper()

	s := newWebSuite(t, withModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Cloud: true,
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.ClientIPRestrictions: {Enabled: true},
			},
		},
	}))

	// The service resolves the client per call, so injecting after startup is fine.
	s.authPlugin.EnableCloud(cloudClient)

	webPack := s.newAuthWebPack(t, "cir-user", withExtraRules(types.NewRule(
		types.KindClientIPRestriction,
		[]string{types.VerbRead, types.VerbList, types.VerbCreate, types.VerbUpdate},
	)))

	return s, webPack, s.testAuthServer.ClusterName()
}

const (
	modeDraft     = cloudapi.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT
	modeEnforced  = cloudapi.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED
	statusPending = cloudapi.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING
	statusActive  = cloudapi.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE
	statusDraft   = cloudapi.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT
)

func cloudCIR(mode cloudapi.ClientIPRestrictionMode, status cloudapi.ClientIPRestrictionStatus, revision string, cidrs ...string) *cloudapi.ClientIPRestriction {
	return &cloudapi.ClientIPRestriction{
		Cidrs:    cidrs,
		Mode:     mode,
		Status:   status,
		Revision: revision,
	}
}

func TestGetClientIPRestriction(t *testing.T) {
	t.Parallel()

	cloudClient := &fakeCloudCIRClient{
		get: func(*cloudapi.GetClientIPRestrictionRequest) (*cloudapi.GetClientIPRestrictionResponse, error) {
			return &cloudapi.GetClientIPRestrictionResponse{
				ClientIpRestriction: cloudCIR(modeEnforced, statusActive, "rev-1", "10.0.0.0/8"),
			}, nil
		},
	}
	s, webPack, clusterName := newCIRTestEnv(t, cloudClient)

	endpoint := webPack.clt.Endpoint("enterprise", "sites", clusterName, "clientiprestriction")
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)

	var got ui.ClientIPRestriction
	require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
	require.Equal(t, ui.ClientIPRestriction{
		Cidrs:    []string{"10.0.0.0/8"},
		Mode:     "enforced",
		Status:   "active",
		Revision: "rev-1",
	}, got)
}

func TestPutClientIPRestriction(t *testing.T) {
	t.Parallel()

	// The revision is what turns the write into a guarded compare-and-set rather
	// than a blind create-or-replace, so which RPC it reaches is the behavior
	// under test.
	t.Run("a revision makes it a guarded update", func(t *testing.T) {
		t.Parallel()

		var updated *cloudapi.UpdateClientIPRestrictionRequest
		cloudClient := &fakeCloudCIRClient{
			update: func(in *cloudapi.UpdateClientIPRestrictionRequest) (*cloudapi.UpdateClientIPRestrictionResponse, error) {
				updated = in
				return &cloudapi.UpdateClientIPRestrictionResponse{
					ClientIpRestriction: cloudCIR(modeDraft, statusDraft, "rev-2", "10.0.0.0/8"),
				}, nil
			},
		}
		s, webPack, clusterName := newCIRTestEnv(t, cloudClient)

		endpoint := webPack.clt.Endpoint("enterprise", "sites", clusterName, "clientiprestriction")
		resp, err := webPack.clt.PutJSON(s.ctx, endpoint, ui.PutClientIPRestrictionRequest{
			Cidrs:    []string{"10.0.0.0/8"},
			Mode:     "draft",
			Revision: "rev-1",
		})
		require.NoError(t, err)

		// The caller's revision has to reach Cloud, or the guard is not applied.
		require.NotNil(t, updated, "expected a guarded update, got none")
		require.Equal(t, "rev-1", updated.Revision)
		require.Equal(t, []string{"10.0.0.0/8"}, updated.Cidrs)
		require.Equal(t, modeDraft, updated.Mode)

		var got ui.ClientIPRestriction
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		require.Equal(t, "rev-2", got.Revision)
	})

	t.Run("no revision makes it an upsert", func(t *testing.T) {
		t.Parallel()

		var upserted *cloudapi.UpsertClientIPRestrictionRequest
		cloudClient := &fakeCloudCIRClient{
			upsert: func(in *cloudapi.UpsertClientIPRestrictionRequest) (*cloudapi.UpsertClientIPRestrictionResponse, error) {
				upserted = in
				return &cloudapi.UpsertClientIPRestrictionResponse{
					ClientIpRestriction: cloudCIR(modeEnforced, statusPending, "rev-1", "10.0.0.0/8"),
				}, nil
			},
		}
		s, webPack, clusterName := newCIRTestEnv(t, cloudClient)

		endpoint := webPack.clt.Endpoint("enterprise", "sites", clusterName, "clientiprestriction")
		resp, err := webPack.clt.PutJSON(s.ctx, endpoint, ui.PutClientIPRestrictionRequest{
			Cidrs: []string{"10.0.0.0/8"},
			Mode:  "enforced",
		})
		require.NoError(t, err)

		// The upsert request carries no revision field at all, which is precisely
		// the difference from the guarded path.
		require.NotNil(t, upserted, "expected an upsert, got none")
		require.Equal(t, []string{"10.0.0.0/8"}, upserted.Cidrs)
		require.Equal(t, modeEnforced, upserted.Mode)

		var got ui.ClientIPRestriction
		require.NoError(t, json.Unmarshal(resp.Bytes(), &got))
		require.Equal(t, "pending", got.Status)
	})

	t.Run("propagates a rejected write", func(t *testing.T) {
		t.Parallel()

		cloudClient := &fakeCloudCIRClient{
			update: func(*cloudapi.UpdateClientIPRestrictionRequest) (*cloudapi.UpdateClientIPRestrictionResponse, error) {
				return nil, trace.CompareFailed("revision does not match")
			},
		}
		s, webPack, clusterName := newCIRTestEnv(t, cloudClient)

		endpoint := webPack.clt.Endpoint("enterprise", "sites", clusterName, "clientiprestriction")
		_, err := webPack.clt.PutJSON(s.ctx, endpoint, ui.PutClientIPRestrictionRequest{
			Cidrs:    []string{"10.0.0.0/8"},
			Revision: "stale",
		})
		require.Error(t, err)
	})
}
