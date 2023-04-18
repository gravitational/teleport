package cloud

import (
	"context"
	"os"

	liblicense "github.com/gravitational/license"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/api/cloud"
	v1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/modules"
)

// FetchFeatures performs a gRPC call to Cloud's tenant service to query
// the features enabled by the licenses's subscription
func FetchFeatures(ctx context.Context, license liblicense.License) (*modules.Features, error) {
	cloudAPIServerAddr := os.Getenv(EnvVarHostPort)
	if cloudAPIServerAddr == "" {
		return nil, trace.BadParameter("license requires fetching features from Cloud but no Cloud host was provided")
	}

	apiServerAddr, err := GetServerAddr(cloudAPIServerAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := liblicense.MakeTLSConfig(license)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig.ServerName = apiServerAddr.Host()
	tlsConfig.InsecureSkipVerify = lib.IsInsecureDevMode()

	cloudClient, err := cloud.NewClient(cloud.ClientConfig{
		Hostname:  apiServerAddr.Addr,
		TLSConfig: tlsConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	features, err := cloudClient.GetFeatures(ctx, &v1.EmptyRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &modules.Features{
		Kubernetes:              features.Kubernetes,
		App:                     features.App,
		DB:                      features.Db,
		Desktop:                 features.Desktop,
		AdvancedAccessWorkflows: features.AccessRequests,
		Cloud:                   features.IsCloud,
		OIDC:                    features.Oidc,
		SAML:                    features.SAML,
		AccessControls:          features.AccessControls,
	}, nil
}
