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

var featuresBackendKey = backend.Key("cloud", "features")

// GetCloudFeatures performs a gRPC call to Cloud's tenant service to query entitlements
func GetCloudFeatures(ctx context.Context, cloudClient cloud.Client) (*modules.Features, error) {
	resp, err := cloudClient.GetFeatures(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f := &modules.Features{
		// Cloud Settings
		Cloud:               resp.IsCloud,
		CustomTheme:         resp.CustomTheme,
		IsStripeManaged:     resp.StripeManaged,
		IsUsageBasedBilling: resp.IsUsageBased,
		Questionnaire:       resp.Questionnaire,
		SupportType:         proto.SupportType(resp.SupportType),

		// Cloud Entitlements
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.AccessLists:            GetCloudEntitlement(resp.Entitlements, entitlements.AccessLists),
			entitlements.AccessMonitoring:       GetCloudEntitlement(resp.Entitlements, entitlements.AccessMonitoring),
			entitlements.AccessRequests:         GetCloudEntitlement(resp.Entitlements, entitlements.AccessRequests),
			entitlements.App:                    GetCloudEntitlement(resp.Entitlements, entitlements.App),
			entitlements.CloudAuditLogRetention: GetCloudEntitlement(resp.Entitlements, entitlements.CloudAuditLogRetention),
			entitlements.DB:                     GetCloudEntitlement(resp.Entitlements, entitlements.DB),
			entitlements.Desktop:                GetCloudEntitlement(resp.Entitlements, entitlements.Desktop),
			entitlements.DeviceTrust:            GetCloudEntitlement(resp.Entitlements, entitlements.DeviceTrust),
			entitlements.ExternalAuditStorage:   GetCloudEntitlement(resp.Entitlements, entitlements.ExternalAuditStorage),
			entitlements.FeatureHiding:          GetCloudEntitlement(resp.Entitlements, entitlements.FeatureHiding),
			entitlements.HSM:                    GetCloudEntitlement(resp.Entitlements, entitlements.HSM),
			entitlements.Identity:               GetCloudEntitlement(resp.Entitlements, entitlements.Identity),
			entitlements.JoinActiveSessions:     GetCloudEntitlement(resp.Entitlements, entitlements.JoinActiveSessions),
			entitlements.K8s:                    GetCloudEntitlement(resp.Entitlements, entitlements.K8s),
			entitlements.MobileDeviceManagement: GetCloudEntitlement(resp.Entitlements, entitlements.MobileDeviceManagement),
			entitlements.OIDC:                   GetCloudEntitlement(resp.Entitlements, entitlements.OIDC),
			entitlements.OktaSCIM:               GetCloudEntitlement(resp.Entitlements, entitlements.OktaSCIM),
			entitlements.OktaUserSync:           GetCloudEntitlement(resp.Entitlements, entitlements.OktaUserSync),
			entitlements.Policy:                 GetCloudEntitlement(resp.Entitlements, entitlements.Policy),
			entitlements.SAML:                   GetCloudEntitlement(resp.Entitlements, entitlements.SAML),
			entitlements.SessionLocks:           GetCloudEntitlement(resp.Entitlements, entitlements.SessionLocks),
			entitlements.UpsellAlert:            GetCloudEntitlement(resp.Entitlements, entitlements.UpsellAlert),
			entitlements.UsageReporting:         GetCloudEntitlement(resp.Entitlements, entitlements.UsageReporting),
		},

		// todo (michellescripts) remove deprecated features
		ProductType:    modules.ProductType(resp.ProductType), // Use entitlements/settings
		AccessControls: true,                                  // AccessControls is true for all customers

		// The following features exist on modules.Features but are not set by Cloud, so they are not set here.
		// AdvancedAccessWorkflows, RecoveryCodes, Plugins, AutomaticUpgrades,
		// The following features are enabled elsewhere if the cluster is configured for that feature
		//AccessGraph, AccessMonitoringConfigured
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
		return nil, trace.Wrap(err, "unmarshalling features")
	}

	return stored, nil
}
