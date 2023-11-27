package feature

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

var featuresBackendKey = backend.Key("cloud", "features")

// FetchFromCloud performs a gRPC call to Cloud's tenant service to query
// the features enabled by the licenses's subscription
func FetchFromCloud(ctx context.Context, cloudClient cloud.Client) (*modules.Features, error) {
	resp, err := cloudClient.GetFeatures(ctx, &v1.EmptyRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f := &modules.Features{
		Kubernetes:              resp.Kubernetes,
		App:                     resp.App,
		DB:                      resp.Db,
		Desktop:                 resp.Desktop,
		AdvancedAccessWorkflows: resp.AccessRequests,
		Cloud:                   resp.IsCloud,
		OIDC:                    resp.Oidc,
		SAML:                    resp.SAML,
		AccessControls:          resp.AccessControls,
		HSM:                     resp.Hsm,
		IsUsageBasedBilling:     resp.IsUsageBased,
		Assist:                  resp.Assist,
		// TODO(codingllama): Pull device trust settings from Cloud?
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true,
		},
	}

	// Set usage-based / Team account limits.
	if f.IsUsageBasedBilling {
		f.DeviceTrust.DevicesUsageLimit = 5
		f.AccessRequests.MonthlyRequestLimit = 5
	}

	return f, nil
}

// Store stores the features in the backend (b)
func Store(ctx context.Context, features modules.Features, b backend.Backend) (*backend.Lease, error) {
	val, err := utils.FastMarshal(features)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b.Put(ctx, backend.Item{
		Key:   featuresBackendKey,
		Value: val,
	})
}

// Load loads features from backend
func Load(ctx context.Context, b backend.Backend) (*modules.Features, error) {
	item, err := b.Get(ctx, featuresBackendKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	stored := &modules.Features{}
	if err := json.Unmarshal(item.Value, stored); err != nil {
		return nil, trace.Wrap(err, "unmarshaling features")
	}

	return stored, nil
}
