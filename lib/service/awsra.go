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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	rolesanywheretypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
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
		pollInterval := time.Second * 20
		resourceLifetime := pollInterval * 2

		// TODO(marco): ensure a single auth instance is running this section
		select {
		case <-ctx.Done():
			return nil

		case <-time.After(pollInterval):
		}

		appServerProfileExpiration := func() *time.Time {
			t := process.Clock.Now().Add(resourceLifetime)
			return &t
		}

		logger.DebugContext(ctx, "Starting AWS Roles Anywhere Profile sync")
		awsRAIntegrations, err := collectAllAWSRAIntegrations(ctx, authClient)
		if err != nil {
			logger.ErrorContext(ctx, "failed to collect AWS Roles Anywhere integrations", "error", err)
			continue
		}

		for _, integration := range awsRAIntegrations {
			logger := logger.With("integration", integration.GetName())

			if integration.GetAWSRAIntegrationSpec().ProfileSyncConfig == nil || !integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.Enabled {
				logger.DebugContext(ctx, "Profile Sync is not enabled")
				continue
			}

			trustAnchorARN := integration.GetAWSRAIntegrationSpec().TrustAnchorARN
			profileSyncProfileARN := integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.ProfileARN
			profileSyncRoleARN := integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.RoleARN
			profileAcceptsRoleSessionName := integration.GetAWSRAIntegrationSpec().ProfileSyncConfig.ProfileAcceptsRoleSessionName

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

			resp, err := awsra.GenerateCredentials(ctx, awsra.GenerateCredentialsRequest{
				Clock:                 process.Clock,
				TrustAnchorARN:        trustAnchorARN,
				ProfileARN:            profileSyncProfileARN,
				RoleARN:               profileSyncRoleARN,
				SubjectCommonName:     "auth-service",
				NotAfter:              time.Now().Add(time.Hour * 1),
				AcceptRoleSessionName: profileAcceptsRoleSessionName,
				CertificateGenerator:  tlsCA,
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

			var allProfiles []rolesanywheretypes.ProfileDetail
			var nextPage *string
			for {
				profilesListResp, err := rolesanywhereClient.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{
					NextToken: nextPage,
				})
				if err != nil {
					logger.ErrorContext(ctx, "failed to rolesanywhere:ListProfiles", "error", err)
					continue
				}
				allProfiles = append(allProfiles, profilesListResp.Profiles...)

				if aws.ToString(profilesListResp.NextToken) == "" {
					break
				}
				nextPage = profilesListResp.NextToken
			}

			for _, profile := range allProfiles {
				logger.DebugContext(ctx, "AWS IAM Roles Anywhere Profile found", "profile_arn", *profile.ProfileArn, "profile_name", *profile.Name)

				if aws.ToString(profile.ProfileArn) == profileSyncProfileARN {
					logger.DebugContext(ctx, "Skipping Profile which is used to sync Profiles into Apps", "profile_arn", *profile.ProfileArn, "profile_name", *profile.Name)
					if profileAcceptsRoleSessionName != aws.ToBool(profile.AcceptRoleSessionName) {
						// TODO(marco): update the integration sync field
					}
					continue
				}
				if !aws.ToBool(profile.Enabled) {
					logger.WarnContext(ctx, "Skipping disabled Profile")
					continue
				}
				if len(profile.RoleArns) == 0 {
					logger.WarnContext(ctx, "Skipping Profile with no Role ARNs")
					continue
				}

				if !strings.HasPrefix(*profile.Name, "MarcoRA") {
					logger.WarnContext(ctx, "Skipping Profile with unexpected name prefix", "profile_name", *profile.Name)
					continue
				}

				labels := make(map[string]string)
				profileTags, err := rolesanywhereClient.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
					ResourceArn: profile.ProfileArn,
				})
				if err != nil {
					logger.WarnContext(ctx, "failed to rolesanywhere:ListProfiles", "error", err)
					continue
				}

				for _, tag := range profileTags.Tags {
					labels["aws/"+aws.ToString(tag.Key)] = aws.ToString(tag.Value)
				}

				// TODO(marco): fix appServer creation (url + new method)
				appURL := "awsaccess.marcoandredinis.com"
				if *profile.Name != "MarcoRA-RO-EC2" {
					appURL = "awsaccess2.marcoandredinis.com"
				}

				appServer, err := types.NewAppServerForAWSOIDCIntegration(*profile.Name, process.Config.HostUUID, appURL, labels)
				if err != nil {
					logger.WarnContext(ctx, "failed to NewAppServerForAWSOIDCIntegration", "error", err)
					continue
				}
				appServer.Metadata.Expires = appServerProfileExpiration()
				appServer.Spec.App.Spec.Integration = integration.GetName()
				appServer.Spec.App.Spec.AWS = &types.AppAWS{
					RolesAnywhereProfile: &types.AppAWSRolesAnywhereProfile{
						ProfileARN:            aws.ToString(profile.ProfileArn),
						AcceptRoleSessionName: aws.ToBool(profile.AcceptRoleSessionName),
					},
				}

				if _, err := authClient.UpsertApplicationServer(ctx, appServer); err != nil {
					logger.WarnContext(ctx, "failed to UpsertApplicationServer", "error", err)
					continue
				}
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
