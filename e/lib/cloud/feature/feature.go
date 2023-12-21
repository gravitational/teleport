package feature

import (
	"context"
	"encoding/json"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/utils"
)

var featuresBackendKey = backend.Key("cloud", "features")

// FetchFromCloud performs a gRPC call to Cloud's tenant service to query
// the features enabled by the licenses's subscription
func FetchFromCloud(ctx context.Context, cloudClient cloud.Client) (*modules.Features, error) {
	resp, err := cloudClient.GetFeatures(ctx, &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Legacy enterprise cloud (non-usage based) will continue to maintain
	// legacy behavior where all (most) features were enabled, and license
	// settings were ignored.
	//
	// Mimic's how we used to set legacy cloud from [func getLicenseFeatures]:
	// https://github.com/gravitational/teleport.e/blob/9b826916ba7d79b1b649286607c358252061f5c5/tool/modules/modules.go#L184
	if isLegacyEnterpiseCloud := !resp.IsUsageBased; isLegacyEnterpiseCloud {
		return &modules.Features{
			Kubernetes:              true,
			App:                     true,
			DB:                      true,
			Desktop:                 true,
			Cloud:                   true,
			OIDC:                    true,
			SAML:                    true,
			AccessControls:          true,
			AdvancedAccessWorkflows: true,
			HSM:                     true,
			RecoveryCodes:           true,
			FeatureHiding:           resp.FeatureHiding,
			CustomTheme:             resp.CustomTheme,
			// Assist is disabled by default.
			Assist: false,
			DeviceTrust: modules.DeviceTrustFeature{
				Enabled: true,
			},

			// New features that are limited even for legacies.
			AccessList:       GetUsageBasedAccessListFeatureLimits(),
			AccessMonitoring: GetUsageBasedAccessMonitoringFeatureLimits(false),
		}, nil
	}

	// From here on, it is usage based billing.

	f := &modules.Features{
		ProductType:                modules.ProductType(resp.ProductType),
		Kubernetes:                 resp.Kubernetes,
		App:                        resp.App,
		DB:                         resp.Db,
		Desktop:                    resp.Desktop,
		AdvancedAccessWorkflows:    resp.AccessRequests,
		Cloud:                      resp.IsCloud,
		OIDC:                       resp.Oidc,
		SAML:                       resp.SAML,
		AccessControls:             resp.AccessControls,
		HSM:                        resp.Hsm,
		IsUsageBasedBilling:        resp.IsUsageBased,
		Assist:                     resp.Assist,
		IdentityGovernanceSecurity: resp.IdentityGovernanceSecurity,
		// The hardcoded values below are used to gate actions from OSS builds.
		DeviceTrust: modules.DeviceTrustFeature{
			Enabled: true,
		},
	}

	// TODO(lisa): these should be set to true from salescenter.
	if resp.ProductType == cloudapi.PRODUCT_TYPE_EUB {
		f.AdvancedAccessWorkflows = true // Gate action from OSS builds.
		f.OIDC = true
		f.SAML = true
		f.AccessControls = true
		f.HSM = true
	}

	// Features will be limited for the following:
	//   1) Team subscriptions
	//   2) EUB subscriptions without IGS enabled
	if resp.ProductType == cloudapi.PRODUCT_TYPE_TEAM || (resp.ProductType == cloudapi.PRODUCT_TYPE_EUB && !resp.IdentityGovernanceSecurity) {
		f.AccessList = GetUsageBasedAccessListFeatureLimits()
		f.AccessRequests = GetUsageBasedAccessRequestFeatureLimits()
		f.DeviceTrust = GetUsageBasedDeviceTrustFeatureLimits()
		// access monitoring enabling will be determined outside of feature reading.
		f.AccessMonitoring = GetUsageBasedAccessMonitoringFeatureLimits(false)
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

// GetUsageBasedAccessListFeatureLimits defines limits for access list
// feature for usage based plans eg: Team or EUB (Enterprise Usage Based).
func GetUsageBasedAccessListFeatureLimits() modules.AccessListFeature {
	return modules.AccessListFeature{
		CreateLimit: 1,
	}
}

// GetUsageBasedAccessRequestFeatureLimits defines limits for access request
// feature for usage based plans eg: Team or EUB (Enterprise Usage Based).
func GetUsageBasedAccessRequestFeatureLimits() modules.AccessRequestsFeature {
	return modules.AccessRequestsFeature{
		MonthlyRequestLimit: 5,
	}
}

// GetUsageBasedDeviceTrustFeatureLimits defines limits for device trust
// feature for usage based plans eg: Team or EUB (Enterprise Usage Based).
func GetUsageBasedDeviceTrustFeatureLimits() modules.DeviceTrustFeature {
	return modules.DeviceTrustFeature{
		Enabled:           true, // always enabled currently
		DevicesUsageLimit: 5,
	}
}

// GetUsageBasedAccessRequestFeatureLimits defines limits for device trust
// feature for usage based plans eg: Team or EUB (Enterprise Usage Based).
func GetUsageBasedAccessMonitoringFeatureLimits(enabled bool) modules.AccessMonitoringFeature {
	return modules.AccessMonitoringFeature{
		Enabled:             enabled,
		MaxReportRangeLimit: 30,
	}
}
