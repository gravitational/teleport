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
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"

	"github.com/gravitational/teleport"
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

// NewGCPWorkforceService creates a new GCPWorkforceService.
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
	fmt.Println("\nConfiguring Workforce Identity Federation pool and SAML provider.")

	iamService, err := iam.NewService(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	workforceService := iam.NewLocationsWorkforcePoolsService(iamService)

	fmt.Println("\nCreating workforce pool: ", s.APIParams.PoolName)
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
		// TODO(sshah): parse through error type for better error handling
		return trace.Wrap(err)
	}
	fmt.Println("Pool created.")
	if !resp.Done {
		// 2 minutes timeout is semi-random decision, chosen based on the fact that
		// when creating workforce pool from the GCP web console, it mentions
		// that the operation could take up to 2 minutes.
		pollCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		if err := waitForPoolStatus(pollCtx, workforceService, poolFullName, s.APIParams.PoolName); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Pool %q is ready for use.\n", s.APIParams.PoolName)
	}

	fmt.Println("\nCreating workforce pool provider: ", s.APIParams.PoolProviderName)
	metadata, err := fetchIdPMetadata(s.APIParams.SAMLIdPMetadataURL, s.HTTPClient)
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
		return trace.Wrap(err)
	}

	if !createProviderResp.Done {
		fmt.Printf("Pool provider %q is created but it may take upto a minute more for this provider to become available.\n\n", s.APIParams.PoolProviderName)
		return nil
	}
	fmt.Println("Pool provider created.")
	fmt.Printf("Pool provider %q is ready for use.\n", s.APIParams.PoolProviderName)
	return nil
}

// waitForPoolStatus waits for pool to come online. Returns immediately if error code is other than
// http.StatusForbidden or when context is canceled with timeout.
func waitForPoolStatus(ctx context.Context, workforceService *iam.LocationsWorkforcePoolsService, poolName, poolDisplayName string) error {
	fmt.Printf("Waiting for pool %q status to become available.\n", poolDisplayName)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pool to be created")
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

				return err
			}

			fmt.Printf("Pool %q found at localtion %q.\n", poolDisplayName, result.Name)
			return nil
		}
	}
}

// fetchIdPMetadata is used to fetch Teleport SAML IdP metadata from
// Teleport proxy. Response is returned without any data validation
// as we expect proxy to return with a valid metadata as long as the proxy
// is running as an enterprise module and with SAML IdP enabled.
func fetchIdPMetadata(metadataURL string, httpClient *http.Client) (string, error) {
	if httpClient == nil {
		return "", trace.BadParameter("missing http client")
	}
	fmt.Println("Fetching Teleport SAML IdP metadata.")
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

	fmt.Println("Fetched Teleport SAML IdP metadata.")
	return string(body), nil
}

func (s *GCPWorkforceService) CheckAndSetDefaults() error {
	if err := s.APIParams.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if s.HTTPClient == nil {
		return trace.BadParameter("param HTTPClient required")
	}
	return nil
}
