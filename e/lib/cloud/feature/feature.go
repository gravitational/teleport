package feature

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

var featuresBackendKey = backend.NewKey("cloud", "features")

// GetCloudFeatures performs a gRPC call to Cloud's tenant service to query entitlements
func GetCloudFeatures(ctx context.Context, cloudClient cloud.Client) (*modules.Features, error) {
	resp, err := cloudClient.GetFeatures(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f := &modules.Features{
		// Cloud Settings
		Cloud:                 resp.IsCloud,
		CustomTheme:           resp.CustomTheme,
		IsStripeManaged:       resp.StripeManaged,
		IsUsageBasedBilling:   resp.IsUsageBased,
		Questionnaire:         resp.Questionnaire,
		SupportType:           proto.SupportType(resp.SupportType),
		CloudAnonymizationKey: resp.CloudAnonymizationKey,
		// Entitlements
		Entitlements: GetCloudEntitlements(resp.Entitlements),

		// todo (michellescripts) remove deprecated features
		ProductType:    modules.ProductType(resp.ProductType), // Use entitlements/settings
		AccessControls: true,                                  // AccessControls is true for all customers

		// The following features exist on modules.Features but are not set by Cloud, so they are not set here.
		// RecoveryCodes, Plugins, AutomaticUpgrades,
		// The following features are enabled elsewhere if the cluster is configured for that feature
		//AccessGraph, AccessMonitoringConfigured
	}

	// todo (michellescripts) replace with AccessRequests
	f.AdvancedAccessWorkflows = f.Entitlements[entitlements.AccessRequests].Enabled

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
		return nil, trace.Wrap(err, "unmarshalling features")
	}

	return stored, nil
}
