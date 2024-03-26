package samlidp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/api/googleapi"
	iam "google.golang.org/api/iam/v1"
)

// ConfigureGCPWorkforcePrams defines input params
// to configure GCP Workforce Identity Federation pool
// and pool provider
type ConfigureGCPWorkforcePrams struct {
	PoolName         string
	PoolProviderName string
	OrganizationID   string
	SAMLIdPMetadata  string
}

// ConfigureGCPWorkforce creates GCP Workforce Identity Federation pool and pool provider
// with the given params.
func ConfigureGCPWorkforce(ctx context.Context, params ConfigureGCPWorkforcePrams) error {
	fmt.Println("\nConfiguring Workforce Identity Federation pool and SAML provider.")

	switch {
	case params.PoolName == "":
		return trace.BadParameter("param PoolName required")
	case params.PoolProviderName == "":
		return trace.BadParameter("param PoolProviderName required")
	case params.OrganizationID == "":
		return trace.BadParameter("param OrganizationID required")
	case params.SAMLIdPMetadata == "":
		return trace.BadParameter("param SAMLIdPMetadata required")
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

	if !resp.Done {
		pollCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		if err := waitForPoolStatus(pollCtx, workforceService, poolFullName, params.PoolName); err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Done creating pool %q.\n", params.PoolName)
	}

	fmt.Println("\nCreating workforce pool provider: ", params.PoolProviderName)
	provider := &iam.WorkforcePoolProvider{
		Description: "pool provider created by Teleport",
		Name:        fmt.Sprintf("locations/global/workforcePools/%s/providers/%s", params.PoolName, params.PoolProviderName),
		DisplayName: params.PoolProviderName,
		Saml: &iam.GoogleIamAdminV1WorkforcePoolProviderSaml{
			IdpMetadataXml: params.SAMLIdPMetadata,
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
		fmt.Printf("Pool provider %s is created but it may take a few seconds more for this provider to be available.\n", params.PoolProviderName)
	}

	fmt.Printf("Done creating pool provider %q.\n", params.PoolProviderName)
	return nil
}

func waitForPoolStatus(ctx context.Context, workforceService *iam.LocationsWorkforcePoolsService, poolName, poolDisplayName string) error {
	fmt.Printf("Waiting for pool %q status to be available.\n", poolDisplayName)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pool to be created")
		case <-ticker.C:
			result, err := workforceService.Get(poolName).Do()
			if err != nil {
				if err.(*googleapi.Error).Code == http.StatusForbidden {
					// StatusForbidden has two meanings:
					// Either the caller does not meet required privilege
					// Or the pool creation is still in progress.
					// We'll continue considering it's the latter.
					continue
				}

				return err
			}

			fmt.Printf("Pool %q found at localtion %q.\n", poolDisplayName, result.Name)
			return nil
		}
	}
}
