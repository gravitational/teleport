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

// ConfigureGCPWorkforce creates GCP Workforce Identity Federation pool and pool provider
// with the given params.
func ConfigureGCPWorkforce(ctx context.Context, params samlidpconfig.GCPWorkforcePrams) error {
	fmt.Println("\nConfiguring Workforce Identity Federation pool and SAML provider.")

	switch {
	case params.PoolName == "":
		return trace.BadParameter("param PoolName required")
	case params.PoolProviderName == "":
		return trace.BadParameter("param PoolProviderName required")
	case params.OrganizationID == "":
		return trace.BadParameter("param OrganizationID required")
	case params.SAMLIdPMetadataURL == "":
		return trace.BadParameter("param SAMLIdPMetadataURL required")
	case params.HTTPClient == nil:
		return trace.BadParameter("param HTTPClient required")
	}

	iamService, err := iam.NewService(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	workforceService := iam.NewLocationsWorkforcePoolsService(iamService)

	fmt.Println("\nCreating workforce pool: ", params.PoolName)
	poolFullName := fmt.Sprintf("locations/global/workforcePools/%s", params.PoolName)
	createPool := workforceService.Create(
		"locations/global",
		&iam.WorkforcePool{
			Name:        poolFullName,
			DisplayName: params.PoolName,
			Description: "pool created by Teleport",
			Parent:      fmt.Sprintf("organizations/%s", params.OrganizationID),
		})
	createPool.WorkforcePoolId(params.PoolName)
	resp, err := createPool.Do()
	if err != nil {
		// TODO(sshah): parse through error type for better error handling
		return trace.Wrap(err)
	}
	fmt.Println("Pool created.")
	if !resp.Done {
		// 2 minutes timeout is semi-random value chosen on the fact that
		// wehen creating workforce pool from the GCP web console, it mentions
		// that the operation could take up to 2 minutes.
		pollCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		if err := waitForPoolStatus(pollCtx, workforceService, poolFullName, params.PoolName); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Pool %q is ready for use.\n", params.PoolName)
	}

	fmt.Println("\nCreating workforce pool provider: ", params.PoolProviderName)
	metadata, err := fetchIdPMetadata(params.SAMLIdPMetadataURL, params.HTTPClient)
	if err != nil {
		return trace.Wrap(err)
	}
	provider := &iam.WorkforcePoolProvider{
		Description: "pool provider created by Teleport",
		Name:        fmt.Sprintf("locations/global/workforcePools/%s/providers/%s", params.PoolName, params.PoolProviderName),
		DisplayName: params.PoolProviderName,
		Saml: &iam.GoogleIamAdminV1WorkforcePoolProviderSaml{
			IdpMetadataXml: metadata,
		},
		AttributeMapping: map[string]string{
			"google.subject": "assertion.subject",
			"google.groups":  "assertion.attributes.roles",
		},
	}

	createProviderReq := workforceService.Providers.Create(poolFullName, provider)
	createProviderReq.WorkforcePoolProviderId(params.PoolProviderName)
	createProviderResp, err := createProviderReq.Do()
	if err != nil {
		return trace.Wrap(err)
	}

	if !createProviderResp.Done {
		fmt.Printf("Pool provider %s is created but it may take upto a minute more for this provider to become available.\n", params.PoolProviderName)
	}
	fmt.Println("Pool provider created.")
	fmt.Printf("Pool provider %q is ready for use.\n", params.PoolProviderName)
	return nil
}

// waitForPoolStatus waits for pool to become online. Returns immidiately if error code is other than
// http.StatusForbidden or when context is canceld with timeout.
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
						// Either the caller does not meet required privilege
						// Or the pool creation is still in progress.
						// We'll continue considering it's the latter.
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
// Teleport proxy. Response is returned without any data validation.
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

	fmt.Println("Fetched SAML IdP metadata.")
	return string(body), nil
}
