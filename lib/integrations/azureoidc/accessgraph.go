package azureoidc

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// singleSignOnMode represents the possible values for `currentSingleSignOnMode` in `adSingleSignOn`
type singleSignOnMode string

const (
	// singleSignOnModeNone indicates that the application does not have SSO set up.
	singleSignOnModeNone singleSignOnMode = "none" //nolint:unused // this serves as documentation of a possible value.
	// singleSignOnModeFederated indicates federated SSO such as SAML.
	singleSignOnModeFederated singleSignOnMode = "federated"
)

// adSingleSignOn represents the response from https://main.iam.ad.ext.azure.com/api/ApplicationSso/{servicePrincipalID}/SingleSignOn
type adSingleSignOn struct {
	CurrentSingleSignOnMode singleSignOnMode `json:"currentSingleSignOnMode"`
}

// tagInfoCache is the format for the file produced by CreateTAGCacheFile.
type tagInfoCache struct {
	AppSsoSettingsCache []*types.PluginEntraIDAppSSOSettings `json:"app_sso_settings_cache"`
}

// getSingleSignOn uses Azure private API to get basic information about an enterprise applications single sign on mode.
func getSingleSignOn(ctx context.Context, token string, servicePrincipalID string) (*adSingleSignOn, error) {
	payload, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", servicePrincipalID, "SingleSignOn"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result adSingleSignOn
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, trace.Wrap(err, "failed to deserialize SingleSignOn")
	}

	return &result, nil
}

// CreateTAGCacheFile populates a file containing the information necessary for Access Graph to analyze Azure SSO.
func CreateTAGCacheFile(ctx context.Context) error {
	graphClient, err := createGraphClient()
	if err != nil {
		return trace.Wrap(err)
	}

	// Get information about enterprise apps
	appResp, err := graphClient.Applications().Get(ctx, nil)
	if err != nil {
		panic(err)
	}

	// Authorize to the private API
	tenantID, err := getTenantID()
	if err != nil {
		return trace.Wrap(err)
	}
	token, err := getPrivateAPIToken(ctx, tenantID)
	if err != nil {
		return trace.Wrap(err)
	}

	cache := &tagInfoCache{}

	for _, app := range appResp.GetValue() {
		appID := app.GetAppId()
		if appID == nil {
			slog.WarnContext(ctx, "app ID is nil", "app", app)
			continue
		}
		sp, err := graphClient.ServicePrincipalsWithAppId(appID).Get(ctx, nil)
		if err != nil {
			slog.ErrorContext(ctx, "could not retrieve service principal", "app_id", *appID, "error", err)
		}
		spID := sp.GetId()
		if spID == nil {
			slog.WarnContext(ctx, "service principal ID is nil", "app_id", *appID)
			continue
		}

		sso, err := getSingleSignOn(ctx, token, *spID)
		if err != nil {
			slog.WarnContext(ctx, "failed to get single sign on data", "app_id", *appID)
			continue
		} else if sso.CurrentSingleSignOnMode != singleSignOnModeFederated {
			slog.DebugContext(ctx, "sso not set up for app, will skip it", "app_id", *appID, "sp_id", *spID)
			continue
		}

		federatedSSOV2, err := privateAPIGet(ctx, token, path.Join("ApplicationSso", *spID, "FederatedSsoV2"))
		if err != nil {
			slog.WarnContext(ctx, "getting federated SSO v2 info failed", "error", err)
			continue
		}

		cache.AppSsoSettingsCache = append(cache.AppSsoSettingsCache, &types.PluginEntraIDAppSSOSettings{
			AppId:          *appID,
			FederatedSsoV2: gzipBytes(federatedSSOV2),
		})
	}

	payload, err := json.Marshal(cache)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(os.WriteFile("cache.json", payload, 0600), "failed to write the TAG cache file")
}

// gzipBytes compresses the given byte slice, returning the result as a new byte slice.
func gzipBytes(src []byte) []byte {
	out := new(bytes.Buffer)
	writer := gzip.NewWriter(out)

	_, err := io.Copy(writer, bytes.NewReader(src))
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}

	err = writer.Close()
	// We do not expect in-memory bytes I/O to fail.
	if err != nil {
		panic(err)
	}
	return out.Bytes()
}
