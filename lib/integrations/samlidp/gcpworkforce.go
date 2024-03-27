/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package samlidp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1" // Migrate to v2 once it starts supporting workforce service.

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/samlidp/samlidpconfig"
	"github.com/gravitational/teleport/lib/utils"
)

// GCPWorkforceService defines workforce service configuration
// parameters.
type GCPWorkforceService struct {
	// APIParams holds basic input params required to create GCP workforce
	// pool and pool provider.
	APIParams samlidpconfig.GCPWorkforceAPIParams
	// HTTPClient is used to fetch metadata from the SAMLIdPMetadataURL
	// endpoint.
	HTTPClient *http.Client
}

// NewGCPWorkforceService creates a new GCPWorkforceService with input validation.
func NewGCPWorkforceService(cfg GCPWorkforceService) (*GCPWorkforceService, error) {
	newGCPWorkforceService := &GCPWorkforceService{
		APIParams: samlidpconfig.GCPWorkforceAPIParams{
			PoolName:           cfg.APIParams.PoolName,
			PoolProviderName:   cfg.APIParams.PoolProviderName,
			OrganizationID:     cfg.APIParams.OrganizationID,
			SAMLIdPMetadataURL: cfg.APIParams.SAMLIdPMetadataURL,
		},
		HTTPClient: cfg.HTTPClient,
	}

	if err := newGCPWorkforceService.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return newGCPWorkforceService, nil
}

// CreateWorkforcePoolAndProvider creates GCP Workforce Identity Federation pool and pool provider
// with the given GCPWorkforceAPIParams values.
func (s *GCPWorkforceService) CreateWorkforcePoolAndProvider(ctx context.Context) error {
	slog.InfoContext(ctx, "Configuring Workforce Identity Federation pool and SAML provider.")

	iamService, err := iam.NewService(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	workforceService := iam.NewLocationsWorkforcePoolsService(iamService)

	slog.With("pool_name", s.APIParams.PoolName).InfoContext(ctx, "Creating workforce pool.")
	poolFullName := fmt.Sprintf("locations/global/workforcePools/%s", s.APIParams.PoolName)
	createPool := workforceService.Create(
		"locations/global",
		&iam.WorkforcePool{
			Name:        poolFullName,
			DisplayName: s.APIParams.PoolName,
			Description: "Workforce pool created by Teleport",
			Parent:      fmt.Sprintf("organizations/%s", s.APIParams.OrganizationID),
		})
	createPool.WorkforcePoolId(s.APIParams.PoolName)
	resp, err := createPool.Do()
	if err != nil {
		if checkGoogleAPIAlreadyExistErr(err) {
			slog.With("pool_name", s.APIParams.PoolName).WarnContext(ctx, "Pool already exists.")
			return s.createWorkforceProvider(ctx, workforceService, poolFullName)
		}
		// TODO(sshah): parse through error type for better error messaging
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Pool created.")
	if !resp.Done {
		// The GCP web console, mentions that the pool create operation could take up to 2 minutes.
		// We will wait for 5 minutes so we have enough time to ensure pool is available.
		pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		if err := waitForPoolStatus(pollCtx, workforceService, poolFullName, s.APIParams.PoolName); err != nil {
			return trace.Wrap(err)
		}
		slog.With("pool_name", s.APIParams.PoolName, "Pool ready for use.")
	}

	return s.createWorkforceProvider(ctx, workforceService, poolFullName)
}

func (s *GCPWorkforceService) createWorkforceProvider(ctx context.Context, workforceService *iam.LocationsWorkforcePoolsService, poolFullName string) error {
	slog.With("pool_provider_name", s.APIParams.PoolProviderName).InfoContext(ctx, "Creating workforce pool provider.")
	metadata, err := fetchIdPMetadata(ctx, s.APIParams.SAMLIdPMetadataURL, s.HTTPClient)
	if err != nil {
		return trace.Wrap(err)
	}
	provider := &iam.WorkforcePoolProvider{
		Description: "Workforce pool provider created by Teleport",
		Name:        fmt.Sprintf("locations/global/workforcePools/%s/providers/%s", s.APIParams.PoolName, s.APIParams.PoolProviderName),
		DisplayName: s.APIParams.PoolProviderName,
		Saml: &iam.GoogleIamAdminV1WorkforcePoolProviderSaml{
			IdpMetadataXml: metadata,
		},
		AttributeMapping: map[string]string{
			"google.subject": "assertion.subject",
			"google.groups":  "assertion.attributes.roles",
		},
	}

	createProviderReq := workforceService.Providers.Create(poolFullName, provider)
	createProviderReq.WorkforcePoolProviderId(s.APIParams.PoolProviderName)
	createProviderResp, err := createProviderReq.Do()
	if err != nil {
		if checkGoogleAPIAlreadyExistErr(err) {
			slog.With("pool_provider_name", s.APIParams.PoolName).ErrorContext(ctx, "Pool provider already exists.")
		}
		return trace.Wrap(err)
	}

	if !createProviderResp.Done {
		slog.With("pool_provider_name", s.APIParams.PoolProviderName).
			InfoContext(ctx, "Pool provider is created but it may take up to a minute more for this provider to become available.")
		return nil
	}
	slog.InfoContext(ctx, "Pool provider created.")
	slog.With("pool_provider_name", s.APIParams.PoolProviderName).InfoContext(ctx, "Pool provider is ready for use.")
	return nil
}

// waitForPoolStatus waits for pool to come online. Returns immediately if error code is other than
// http.StatusForbidden or when context is canceled with timeout.
func waitForPoolStatus(ctx context.Context, workforceService *iam.LocationsWorkforcePoolsService, poolName, poolDisplayName string) error {
	slog.InfoContext(ctx, "Waiting for pool status. It may take up to 5 minutes for new pool to become available.")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf(`timeout while waiting for pool to be available. If you can confirm that the pool is already created, rerun the command again`)
		case <-ticker.C:
			result, err := workforceService.Get(poolName).Do()
			if err != nil {
				var googleApiErr *googleapi.Error
				if errors.As(err, &googleApiErr) {
					if googleApiErr.Code == http.StatusForbidden {
						// StatusForbidden has two meanings:
						// Either the caller does not meet required privilege.
						// Or the pool creation is still in progress.
						// We'll continue considering it's the latter case.
						continue
					}
				}
				return trace.Wrap(err)
			}

			if result.State == "ACTIVE" {
				slog.With("pool_name", poolDisplayName).InfoContext(ctx, "Pool found.")
				return nil
			}
		}
	}
}

// fetchIdPMetadata is used to fetch Teleport SAML IdP metadata from
// Teleport proxy. Response is returned without any data validation
// as we expect proxy to return with a valid metadata as long as the proxy
// is running as an enterprise module and with SAML IdP enabled.
func fetchIdPMetadata(ctx context.Context, metadataURL string, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		return "", trace.BadParameter("missing http client")
	}
	slog.InfoContext(ctx, "Fetching Teleport SAML IdP metadata.")
	resp, err := httpClient.Get(metadataURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", trace.BadParameter("IdP metadata not found at given URL.")
	}

	body, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPResponseSize)
	if err != nil {
		return "", trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Fetched Teleport SAML IdP metadata.")
	return string(body), nil
}

func (s *GCPWorkforceService) CheckAndSetDefaults() error {
	if err := s.APIParams.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.HTTPClient == nil {
		httpClient, err := defaults.HTTPClient()
		if err != nil {
			return trace.Wrap(err)
		}
		s.HTTPClient = httpClient
		// we expect metadata to be available at the given SAMLIdPMetadataURL endpoint.
		// As such client should be configured to not to follow redirect response.
		s.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return nil
}

func checkGoogleAPIAlreadyExistErr(err error) bool {
	var googleApiErr *googleapi.Error
	if errors.As(err, &googleApiErr) {
		if googleApiErr.Code == http.StatusConflict {
			return true
		}
	}
	return false
}
