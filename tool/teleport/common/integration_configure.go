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

package common

import (
	"context"
	"fmt"
	"log/slog"

	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	ecatypes "github.com/gravitational/teleport/api/types/externalauditstorage"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	"github.com/gravitational/teleport/lib/integrations/azureoidc"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage"
	"github.com/gravitational/teleport/lib/integrations/externalauditstorage/easconfig"
	"github.com/gravitational/teleport/lib/integrations/samlidp"
	"github.com/gravitational/teleport/lib/integrations/samlidp/samlidpconfig"
	"github.com/gravitational/teleport/lib/utils"
)

func onIntegrationConfDeployService(ctx context.Context, params config.IntegrationConfDeployServiceIAM) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	iamClient, err := awsoidc.NewDeployServiceIAMConfigureClient(ctx, params.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.DeployServiceIAMConfigureRequest{
		AccountID:       params.AccountID,
		Cluster:         params.Cluster,
		IntegrationName: params.Name,
		Region:          params.Region,
		IntegrationRole: params.Role,
		TaskRole:        params.TaskRole,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureDeployServiceIAM(ctx, iamClient, confReq))
}

func onIntegrationConfEC2SSMIAM(ctx context.Context, params config.IntegrationConfEC2SSMIAM) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	awsClt, err := awsoidc.NewEC2SSMConfigureClient(ctx, params.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.EC2SSMIAMConfigureRequest{
		Region:          params.Region,
		IntegrationRole: params.RoleName,
		SSMDocumentName: params.SSMDocumentName,
		ProxyPublicURL:  params.ProxyPublicURL,
		ClusterName:     params.ClusterName,
		IntegrationName: params.IntegrationName,
		AccountID:       params.AccountID,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureEC2SSM(ctx, awsClt, confReq))
}

func onIntegrationConfAWSAppAccessIAM(ctx context.Context, params config.IntegrationConfAWSAppAccessIAM) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	iamClient, err := awsoidc.NewAWSAppAccessConfigureClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.AWSAppAccessConfigureRequest{
		IntegrationRole: params.RoleName,
		AccountID:       params.AccountID,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureAWSAppAccess(ctx, iamClient, confReq))
}

func onIntegrationConfEKSIAM(ctx context.Context, params config.IntegrationConfEKSIAM) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	iamClient, err := awsoidc.NewEKSIAMConfigureClient(ctx, params.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.EKSIAMConfigureRequest{
		Region:          params.Region,
		IntegrationRole: params.Role,
		AccountID:       params.AccountID,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureEKSIAM(ctx, iamClient, confReq))
}

func onIntegrationConfAWSOIDCIdP(ctx context.Context, clf config.CommandLineFlags) error {
	// pass the value of --insecure flag to the runtime
	lib.SetInsecureDevMode(clf.InsecureMode)

	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	iamClient, err := awsoidc.NewIdPIAMConfigureClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.IdPIAMConfigureRequest{
		Cluster:                 clf.IntegrationConfAWSOIDCIdPArguments.Cluster,
		IntegrationName:         clf.IntegrationConfAWSOIDCIdPArguments.Name,
		IntegrationRole:         clf.IntegrationConfAWSOIDCIdPArguments.Role,
		ProxyPublicAddress:      clf.IntegrationConfAWSOIDCIdPArguments.ProxyPublicURL,
		IntegrationPolicyPreset: awsoidc.PolicyPreset(clf.IntegrationConfAWSOIDCIdPArguments.PolicyPreset),
		AutoConfirm:             clf.IntegrationConfAWSOIDCIdPArguments.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureIdPIAM(ctx, iamClient, confReq))
}

func onIntegrationConfListDatabasesIAM(ctx context.Context, params config.IntegrationConfListDatabasesIAM) error {
	// Ensure we show progress to the user.
	// LogLevel at this point is set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	clt, err := awsoidc.NewListDatabasesIAMConfigureClient(ctx, params.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.ConfigureIAMListDatabasesRequest{
		Region:          params.Region,
		IntegrationRole: params.Role,
		AccountID:       params.AccountID,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureListDatabasesIAM(ctx, clt, confReq))
}

func onIntegrationConfExternalAuditCmd(ctx context.Context, params easconfig.ExternalAuditStorageConfiguration) error {
	cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(params.Region))
	if err != nil {
		return trace.Wrap(err)
	}

	if params.AccountID != "" {
		stsClient := sts.NewFromConfig(cfg)
		err = awsoidc.CheckAccountID(ctx, stsClient, params.AccountID)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if params.Bootstrap {
		err = externalauditstorage.BootstrapInfra(ctx, externalauditstorage.BootstrapInfraParams{
			Athena: athena.NewFromConfig(cfg),
			Glue:   glue.NewFromConfig(cfg),
			S3:     s3.NewFromConfig(cfg),
			Spec: &ecatypes.ExternalAuditStorageSpec{
				SessionRecordingsURI:   params.SessionRecordingsURI,
				AuditEventsLongTermURI: params.AuditEventsURI,
				AthenaResultsURI:       params.AthenaResultsURI,
				AthenaWorkgroup:        params.AthenaWorkgroup,
				GlueDatabase:           params.GlueDatabase,
				GlueTable:              params.GlueTable,
			},
			Region:          params.Region,
			ClusterName:     params.ClusterName,
			IntegrationName: params.IntegrationName,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	clt := &awsoidc.DefaultConfigureExternalAuditStorageClient{
		Iam: iam.NewFromConfig(cfg),
		Sts: sts.NewFromConfig(cfg),
	}
	return trace.Wrap(awsoidc.ConfigureExternalAuditStorage(ctx, clt, params))
}

func onIntegrationConfAccessGraphAWSSync(ctx context.Context, params config.IntegrationConfAccessGraphAWSSync) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	clt, err := awsoidc.NewAccessGraphIAMConfigureClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.AccessGraphAWSIAMConfigureRequest{
		IntegrationRole: params.Role,
		AccountID:       params.AccountID,
		AutoConfirm:     params.AutoConfirm,
	}
	return trace.Wrap(awsoidc.ConfigureAccessGraphSyncIAM(ctx, clt, confReq))
}

func onIntegrationConfAccessGraphAzureSync(ctx context.Context, params config.IntegrationConfAccessGraphAzureSync) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)
	confReq := azureoidc.AccessGraphAzureConfigureRequest{
		ManagedIdentity: params.ManagedIdentity,
		RoleName:        params.RoleName,
		SubscriptionID:  params.SubscriptionID,
		AutoConfirm:     params.AutoConfirm,
	}
	clt, err := azureoidc.NewAzureConfigClient(params.SubscriptionID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(azureoidc.ConfigureAccessGraphSyncAzure(ctx, clt, confReq))
}

func onIntegrationConfAzureOIDCCmd(ctx context.Context, params config.IntegrationConfAzureOIDC) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	if err := azureoidc.EnsureAZLogin(ctx); err != nil {
		return trace.Wrap(err)
	}

	fmt.Println("Teleport is setting up the Azure integration. This may take a few minutes.")

	appID, tenantID, err := azureoidc.SetupEnterpriseApp(ctx, params.ProxyPublicAddr, params.AuthConnectorName, params.SkipOIDCConfiguration)
	if err != nil {
		return trace.Wrap(err)
	}

	if params.AccessGraphEnabled {
		err := azureoidc.CreateTAGCacheFile(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Println()
	fmt.Println("Success! Use the following information to finish the integration onboarding in Teleport:")
	fmt.Printf("Tenant ID: %s\nClient ID: %s\n", tenantID, appID)
	if params.AccessGraphEnabled {
		fmt.Println("To finish the setup you will need the `cache.json` file that we created for you.")
		fmt.Println("Use `download cache.json` to download it from the Azure Cloud Shell, and submit it on the integration onboarding page.")
	}
	return nil
}

func onIntegrationConfSAMLIdPGCPWorkforce(ctx context.Context, params samlidpconfig.GCPWorkforceAPIParams) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	gcpWorkforceService, err := samlidp.NewGCPWorkforceService(samlidp.GCPWorkforceService{
		APIParams: params,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(gcpWorkforceService.CreateWorkforcePoolAndProvider(ctx))
}
