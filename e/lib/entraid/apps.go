package entraid

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"

	"github.com/gravitational/trace"
	samltypes "github.com/russellhaering/gosaml2/types"

	"github.com/gravitational/teleport/api/types"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/msgraph"
)

func entraAppToProto(ctx context.Context, app *msgraph.Application, ssoSettings *types.PluginEntraIDAppSSOSettings, tenantID string, signingCerts []string) (*accessgraphv1alpha.EntraApplication, error) {
	id := app.ID
	if id == nil {
		return nil, trace.BadParameter("expected ID to be present")
	}
	appID := app.AppID
	if appID == nil {
		return nil, trace.BadParameter("expected app ID to be present")
	}
	displayName := app.DisplayName
	if displayName == nil {
		return nil, trace.BadParameter("expected display name to be present")
	}

	var federatedSSOV2 string
	if len(ssoSettings.FederatedSsoV2) > 0 {
		decompressed, err := gunzip(ssoSettings.FederatedSsoV2)
		if err == nil {
			federatedSSOV2 = string(decompressed)
		} else {
			slog.WarnContext(ctx, "failed to decompress FederatedSSOV2", "error", err)
		}
	}

	return &accessgraphv1alpha.EntraApplication{
		Id:                  *id,
		AppId:               *appID,
		DisplayName:         *displayName,
		TenantId:            tenantID,
		SigningCertificates: signingCerts,
		FederatedSsoV2:      federatedSSOV2,
	}, nil
}

func FederationMetadataURL(tenantID, appID string) string {
	return (&url.URL{
		Scheme: "https",
		Host:   "login.microsoftonline.com",
		Path:   path.Join(tenantID, "federationmetadata", "2007-06", "federationmetadata.xml"),
		RawQuery: url.Values{
			"appid": {appID},
		}.Encode(),
	}).String()
}

func getAppSAMLSigningCertificates(ctx context.Context, client *http.Client, tenantID, appID string) ([]string, error) {
	uri := FederationMetadataURL(tenantID, appID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	var metadata samltypes.EntityDescriptor
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&metadata); err != nil {
		return nil, trace.Wrap(err)
	}

	const signingUse = "signing"
	var signingCerts []string
	for _, key := range metadata.IDPSSODescriptor.KeyDescriptors {
		ki := key.KeyInfo
		if key.Use == signingUse {
			for _, cert := range ki.X509Data.X509Certificates {
				signingCerts = append(signingCerts, cert.Data)
			}
		}
	}

	return signingCerts, nil
}

func gunzip(payload []byte) ([]byte, error) {
	src := bytes.NewReader(payload)
	r, err := gzip.NewReader(src)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Close()
	result, err := io.ReadAll(r)
	return result, trace.Wrap(err)
}
