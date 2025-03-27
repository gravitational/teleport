/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package service

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsra"
	"github.com/gravitational/teleport/lib/tlsca"
)

func (process *TeleportProcess) initAWSRAProfileSync() error {
	ctx := process.GracefulExitContext()
	logger := process.logger.With("process", "awsra-profile-sync")
	// start process only after teleport process has started
	if _, err := process.WaitForEvent(ctx, TeleportReadyEvent); err != nil {
		return trace.Wrap(err)
	}

	authClient := process.localAuth
	if authClient == nil {
		return trace.Errorf("instance client not yet initialized")
	}

	for {

		select {
		case <-ctx.Done():
			// TODO(marco): use a configurable poll interval
		case <-time.After(time.Second * 10):
		}

		logger.InfoContext(ctx, "Starting AWS Roles Anywhere Profile sync")
		awsRAIntegrations, err := collectAllAWSRAIntegrations(ctx, authClient)
		if err != nil {
			logger.ErrorContext(ctx, "failed to collect AWS Roles Anywhere integrations", "error", err)
			continue
		}

		for _, integration := range awsRAIntegrations {
			logger := logger.With("integration", integration.GetName())

			if integration.GetAWSRAIntegrationSpec().ProfileSyncConfig == nil || !integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.Enabled {
				logger.InfoContext(ctx, "Skipping because profile sync is not enabled")
				continue
			}

			trustAnchorARN := integration.GetAWSRAIntegrationSpec().TrustAnchorARN
			profileSyncProfileARN := integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.ProfileARN
			profileSyncRoleARN := integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.RoleARN

			ctx := process.GracefulExitContext()
			awsRACA, err := authClient.GetCertAuthority(ctx, types.CertAuthID{
				Type:       types.AWSRACA,
				DomainName: process.instanceConnector.clusterName,
			}, true)
			if err != nil {
				return trace.Wrap(err)
			}

			tlsCert, tlsSigner, err := authClient.GetKeyStore().GetTLSCertAndSigner(ctx, awsRACA)
			if err != nil {
				return trace.Wrap(err)
			}

			tlsCA, err := tlsca.FromCertAndSigner(tlsCert, tlsSigner)
			if err != nil {
				return trace.Wrap(err)
			}

			resp, err := awsra.GenerateAWSRACredentials(ctx, awsra.GenerateAWSRACredentialsRequest{
				Clock:                process.Clock,
				TrustAnchorARN:       trustAnchorARN,
				ProfileARN:           profileSyncProfileARN,
				RoleARN:              profileSyncRoleARN,
				SubjectCommonName:    "auth-service",
				NotAfter:             time.Now().Add(time.Hour * 1),
				CertificateGenerator: tlsCA,
			})
			if err != nil {
				logger.ErrorContext(ctx, "failed to GenerateAWSRACredentials", "error", err)
				continue
			}

			parsedProfileSyncProfile, err := arn.Parse(profileSyncProfileARN)
			if err != nil {
				logger.ErrorContext(ctx, "failed to parse profile arn", "profile_arn", profileSyncProfileARN, "error", err)
				continue
			}
			region := parsedProfileSyncProfile.Region

			awsConfig, err := config.LoadDefaultConfig(
				ctx,
				config.WithRegion(region),
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(resp.AccessKeyID, resp.SecretAccessKey, resp.SessionToken)),
			)
			if err != nil {
				logger.ErrorContext(ctx, "failed to load aws default config", "error", err)
				continue
			}

			rolesanywhereClient := rolesanywhere.NewFromConfig(awsConfig)

			// TODO(marco): handle pagination
			profilesListResp, err := rolesanywhereClient.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{})
			if err != nil {
				logger.ErrorContext(ctx, "failed to rolesanywhere:ListProfiles", "error", err)
				continue
			}

			for _, profile := range profilesListResp.Profiles {
				logger.InfoContext(ctx, "IAM Roles Anywhere Profile found", "profile_arn", *profile.ProfileArn, "profile_name", *profile.Name)

				if aws.ToString(profile.ProfileArn) == profileSyncProfileARN {
					logger.InfoContext(ctx, "Skipping Integration Sync Profile")
					continue
				}
				if !aws.ToBool(profile.Enabled) {
					logger.InfoContext(ctx, "Skipping disabled Profile")
					continue
				}
				if len(profile.RoleArns) == 0 {
					logger.InfoContext(ctx, "Skipping Profile with no Role ARNs")
					continue
				}

				labels := make(map[string]string)
				profileTags, err := rolesanywhereClient.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
					ResourceArn: profile.ProfileArn,
				})
				if err != nil {
					logger.ErrorContext(ctx, "failed to rolesanywhere:ListProfiles", "error", err)
					continue
				}

				for _, tag := range profileTags.Tags {
					labels["aws/"+aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}

				// TODO(marco): fix me
				appURL := "awsaccess.marcoandredinis.com"
				if *profile.Name != "MarcoRA-RO-EC2" {
					appURL = "awsaccess2.marcoandredinis.com"
				}

				appServer, err := types.NewAppServerForAWSOIDCIntegration(*profile.Name, process.Config.HostUUID, appURL, labels)
				if err != nil {
					logger.ErrorContext(ctx, "failed to NewAppServerForAWSOIDCIntegration", "error", err)
					continue
				}
				appServer.Spec.App.Spec.Integration = integration.GetName()
				appServer.Spec.App.Spec.AWS = &types.AppAWS{
					RolesAnywhere: &types.AppAWSRolesAnywhere{
						ProfileARN:      *profile.ProfileArn,
						AllowedRolesARN: profile.RoleArns,
					},
				}

				if _, err := authClient.UpsertApplicationServer(ctx, appServer); err != nil {
					logger.ErrorContext(ctx, "failed to UpsertApplicationServer", "error", err)
					continue
				}
				logger.InfoContext(ctx, "Upserted Application Server", "server", appServer.GetName())
			}

		}
	}
}

func collectAllAWSRAIntegrations(ctx context.Context, integrationListerClient interface {
	ListIntegrations(ctx context.Context, pageSize int, nextKey string) ([]types.Integration, string, error)
}) ([]types.Integration, error) {
	var integrations []types.Integration
	var nextKey string

	for {
		resp, respNextKey, err := integrationListerClient.ListIntegrations(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, integration := range resp {
			if integration.GetSubKind() != types.IntegrationSubKindAWSRA {
				continue
			}
			integrations = append(integrations, integration)
		}
		nextKey = respNextKey
		if nextKey == "" {
			break
		}
	}
	return integrations, nil
}
