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
		Cluster:         params.Cluster,
		IntegrationName: params.Name,
		Region:          params.Region,
		IntegrationRole: params.Role,
		TaskRole:        params.TaskRole,
	}
	return trace.Wrap(awsoidc.ConfigureDeployServiceIAM(ctx, iamClient, confReq))
}

func onIntegrationConfEICEIAM(ctx context.Context, params config.IntegrationConfEICEIAM) error {
	// Ensure we print output to the user. LogLevel at this point was set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	iamClient, err := awsoidc.NewEICEIAMConfigureClient(ctx, params.Region)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.EICEIAMConfigureRequest{
		Region:          params.Region,
		IntegrationRole: params.Role,
	}
	return trace.Wrap(awsoidc.ConfigureEICEIAM(ctx, iamClient, confReq))
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
		Cluster:            clf.IntegrationConfAWSOIDCIdPArguments.Cluster,
		IntegrationName:    clf.IntegrationConfAWSOIDCIdPArguments.Name,
		IntegrationRole:    clf.IntegrationConfAWSOIDCIdPArguments.Role,
		ProxyPublicAddress: clf.IntegrationConfAWSOIDCIdPArguments.ProxyPublicURL,
		S3BucketLocation:   clf.IntegrationConfAWSOIDCIdPArguments.S3BucketURI,
		S3JWKSContentsB64:  clf.IntegrationConfAWSOIDCIdPArguments.S3JWKSContentsB64,
	}
	return trace.Wrap(awsoidc.ConfigureIdPIAM(ctx, iamClient, confReq))
}

func onIntegrationConfListDatabasesIAM(ctx context.Context, params config.IntegrationConfListDatabasesIAM) error {
	// Ensure we show progress to the user.
	// LogLevel at this point is set to Error.
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelInfo)

	if params.Region == "" {
		return trace.BadParameter("region is required")
	}

	cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(params.Region))
	if err != nil {
		return trace.Wrap(err)
	}

	iamClient := iam.NewFromConfig(cfg)

	confReq := awsoidc.ConfigureIAMListDatabasesRequest{
		Region:          params.Region,
		IntegrationRole: params.Role,
	}
	return trace.Wrap(awsoidc.ConfigureListDatabasesIAM(ctx, iamClient, confReq))
}

func onIntegrationConfExternalAuditCmd(ctx context.Context, params easconfig.ExternalAuditStorageConfiguration) error {
	cfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(params.Region))
	if err != nil {
		return trace.Wrap(err)
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
			Region: params.Region,
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

	iamClient, err := awsoidc.NewAccessGraphIAMConfigureClient(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	confReq := awsoidc.AccessGraphAWSIAMConfigureRequest{
		IntegrationRole: params.Role,
	}
	return trace.Wrap(awsoidc.ConfigureAccessGraphSyncIAM(ctx, iamClient, confReq))
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
