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

package awsra

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/utils"
)

// AWSRolesAnywherProfileSyncerParams contains the parameters for the AWS Roles Anywhere Profile Syncer.
type AWSRolesAnywherProfileSyncerParams struct {
	// Clock is used to calculate the expiration time of the AppServers.
	Clock clockwork.Clock

	// Logger is used to log messages.
	Logger *slog.Logger

	// KeyStoreManager grants access to the AWS Roles Anywhere signer.
	KeyStoreManager KeyStoreManager

	// Cache is used to get the current cluster name and cert authority keys.
	Cache SyncerCache

	// AppServerUpserter is used to upsert AppServers.
	AppServerUpserter AppServerUpserter

	// HostUUID is the Host UUID to assign to the AppServers.
	HostUUID string

	// SyncPollInterval is the interval at which to poll for new profiles.
	// Default is 5 minutes.
	SyncPollInterval time.Duration

	// subjectName is the name of the subject to use when generating AWS credentials.
	subjectName string

	// rolesAnywhereClient is the AWS Roles Anywhere client used to interact with the AWS IAM Roles Anywhere service.
	// Should only be set for testing purposes.
	rolesAnywhereClient RolesAnywhereClient

	// createSession is the API used to create a session with AWS IAM Roles Anywhere.
	// This is used to mock the CreateSession API in tests.
	createSession func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error)
}

// SyncerCache is the subset of the cached resources that the syncer service queries.
type SyncerCache interface {
	// GetCertAuthority returns cert authority by id
	GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error)
	// GetClusterName returns the current cluster name.
	GetClusterName(ctx context.Context) (types.ClusterName, error)
	// GetProxies returns a list of proxy servers registered in the cluster
	GetProxies() ([]types.Server, error)
	// ListIntegrations returns a paginated list of all integration resources.
	ListIntegrations(ctx context.Context, pageSize int, nextKey string) ([]types.Integration, string, error)
}

// RolesAnywhereClient is an interface that defines methods to interact with the AWS IAM Roles Anywhere service.
type RolesAnywhereClient interface {
	// Lists all profiles in the authenticated account and Amazon Web Services Region.
	ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error)

	// Lists the tags attached to the resource.
	ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error)
}

func (p *AWSRolesAnywherProfileSyncerParams) checkAndSetDefaults() error {
	if p.KeyStoreManager == nil {
		return trace.BadParameter("key store manager is required")
	}

	if p.Cache == nil {
		return trace.BadParameter("cache client is required")
	}

	if p.AppServerUpserter == nil {
		return trace.BadParameter("app server upserter is required")
	}

	if p.SyncPollInterval == 0 {
		p.SyncPollInterval = 5 * time.Minute
	}

	if p.Logger == nil {
		p.Logger = slog.Default()
	}

	if p.Clock == nil {
		p.Clock = clockwork.NewRealClock()
	}

	if p.HostUUID == "" {
		p.HostUUID = uuid.NewString()
	}

	p.subjectName = "teleport-roles-anywhere-profile-sync"

	return nil
}

// AppServerUpserter is an interface that defines methods for upserting application servers.
type AppServerUpserter interface {
	// UpsertApplicationServer registers an application server.
	UpsertApplicationServer(ctx context.Context, server types.AppServer) (*types.KeepAlive, error)
}

// RunAWSRolesAnywherProfileSyncer starts the AWS Roles Anywhere Profile Syncer.
// It will iterate over all AWS IAM Roles Anywhere integrations, and for each one:
// 1. Check if the Profile Sync is enabled.
// 2. Generate AWS credentials using the integration.
// 3. List all profiles in the AWS IAM Roles Anywhere service.
// 4. For each profile, check if it is enabled and has associated roles.
// 5. Create an AppServer for each profile, using the profile name as the AppServer name.
// AppServer name can be overridden by the `TeleportApplicationName` tag on the Profile.
func RunAWSRolesAnywherProfileSyncer(ctx context.Context, params AWSRolesAnywherProfileSyncerParams) error {
	if err := params.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	for {
		integrations, err := integrationsWithProfileSyncEnabled(ctx, params.Cache)
		if err != nil {
			return trace.Wrap(err)
		}

		proxyPublicAddr, err := fetchProxyPublicAddr(params.Cache)
		if err != nil {
			return trace.Wrap(err)
		}

		for _, integration := range integrations {
			if err := syncProfileForIntegration(ctx, params, integration, proxyPublicAddr); err != nil {
				params.Logger.ErrorContext(ctx, "failed to sync AWS Roles Anywhere Profiles for integration", "error", err)
			}
		}

		select {
		case <-ctx.Done():
			return nil

		case <-params.Clock.After(params.SyncPollInterval):
		}
	}
}

func fetchProxyPublicAddr(cache SyncerCache) (string, error) {
	proxies, err := cache.GetProxies()
	if err != nil {
		return "", trace.Wrap(err)
	}

	if len(proxies) == 0 {
		return "", trace.NotFound("no proxies found")
	}

	proxy := proxies[0]
	if proxy.GetPublicAddr() == "" {
		return "", trace.NotFound("proxy %q does not have a public address", proxy.GetName())
	}
	return proxy.GetPublicAddr(), nil
}

func integrationsWithProfileSyncEnabled(ctx context.Context, cache SyncerCache) ([]types.Integration, error) {
	var integrations []types.Integration
	var nextKey string

	for {
		resp, respNextKey, err := cache.ListIntegrations(ctx, 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, integration := range resp {
			if integration.GetSubKind() != types.IntegrationSubKindAWSRolesAnywhere ||
				integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig == nil ||
				!integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.Enabled {

				continue
			}

			integrations = append(integrations, integration)
		}

		if respNextKey == "" {
			break
		}
		nextKey = respNextKey
	}

	return integrations, nil
}

func buildAWSRolesAnywhereClientForIntegration(ctx context.Context, params AWSRolesAnywherProfileSyncerParams, integration types.Integration) (RolesAnywhereClient, error) {
	trustAnchorARN := integration.GetAWSRolesAnywhereIntegrationSpec().TrustAnchorARN
	profileSyncProfileARN := integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.ProfileARN
	profileSyncRoleARN := integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.RoleARN
	profileAcceptsRoleSessionName := integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.ProfileAcceptsRoleSessionName

	parsedProfileSyncProfile, err := arn.Parse(profileSyncProfileARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	region := parsedProfileSyncProfile.Region

	resp, err := GenerateCredentials(ctx, GenerateCredentialsRequest{
		Clock:                 params.Clock,
		TrustAnchorARN:        trustAnchorARN,
		ProfileARN:            profileSyncProfileARN,
		RoleARN:               profileSyncRoleARN,
		SubjectCommonName:     params.subjectName,
		AcceptRoleSessionName: profileAcceptsRoleSessionName,
		KeyStoreManager:       params.KeyStoreManager,
		Cache:                 params.Cache,
		CreateSession:         params.createSession,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsConfig, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(resp.AccessKeyID, resp.SecretAccessKey, resp.SessionToken)),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// If a custom client is provided, use it instead.
	// This must only be used for testing purposes.
	if params.rolesAnywhereClient != nil {
		return params.rolesAnywhereClient, nil
	}

	return rolesanywhere.NewFromConfig(awsConfig), nil
}

func syncProfileForIntegration(ctx context.Context, params AWSRolesAnywherProfileSyncerParams, integration types.Integration, proxyPublicAddr string) error {
	logger := params.Logger.With("integration", integration.GetName())

	raClient, err := buildAWSRolesAnywhereClientForIntegration(ctx, params, integration)
	if err != nil {
		return trace.Wrap(err)
	}

	var nextPage *string
	for {
		profilesListResp, err := raClient.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{
			NextToken: nextPage,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		for _, profile := range profilesListResp.Profiles {
			err := processProfile(ctx, processProfileRequest{
				Params:          params,
				Profile:         profile,
				RAClient:        raClient,
				Integration:     integration,
				ProxyPublicAddr: proxyPublicAddr,
			})
			if err != nil {
				if errors.Is(err, errDisabledProfile) || errors.Is(err, errProfileIsUsedForSync) {
					logger.DebugContext(ctx, "Skipping profile", "profile_name", aws.ToString(profile.Name), "error", err.Error())
					continue
				}

				logger.WarnContext(ctx, "Failed to process profile", "profile_name", aws.ToString(profile.Name), "error", err)
			}
		}

		if aws.ToString(profilesListResp.NextToken) == "" {
			break
		}
		nextPage = profilesListResp.NextToken
	}

	return nil
}

var (
	errDisabledProfile      = errors.New("profile is disabled")
	errProfileIsUsedForSync = errors.New("profile is used to sync profiles and will not be added as an aws app")
)

type processProfileRequest struct {
	Params          AWSRolesAnywherProfileSyncerParams
	Profile         ratypes.ProfileDetail
	RAClient        RolesAnywhereClient
	Integration     types.Integration
	ProxyPublicAddr string
}

func processProfile(ctx context.Context, req processProfileRequest) error {
	profileSyncProfileARN := req.Integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.ProfileARN

	if aws.ToString(req.Profile.ProfileArn) == profileSyncProfileARN {
		return errProfileIsUsedForSync
	}

	if !aws.ToBool(req.Profile.Enabled) {
		return errDisabledProfile
	}

	profileTags, err := req.RAClient.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
		ResourceArn: req.Profile.ProfileArn,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	appServer, err := convertProfile(req.Params, req.Profile, req.Integration.GetName(), profileTags.Tags, req.ProxyPublicAddr)
	if err != nil {
		return trace.BadParameter("failed to convert Profile to AppServer: %v", err)
	}

	if _, err := req.Params.AppServerUpserter.UpsertApplicationServer(ctx, appServer); err != nil {
		return trace.BadParameter("failed to upsert application server from Profile: %v", err)
	}

	return nil
}

func convertProfile(params AWSRolesAnywherProfileSyncerParams, profile ratypes.ProfileDetail, integrationName string, profileTags []ratypes.Tag, proxyPublicAddr string) (types.AppServer, error) {
	profileName := aws.ToString(profile.Name)
	profileARN := aws.ToString(profile.ProfileArn)
	parsedProfileARN, err := arn.Parse(profileARN)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applicationName := profileName + "-" + integrationName

	labels := make(map[string]string, len(profileTags))
	for _, tag := range profileTags {
		labels["aws/"+aws.ToString(tag.Key)] = aws.ToString(tag.Value)

		if aws.ToString(tag.Key) == types.AWSRolesAnywhereProfileNameOverrideLabel {
			applicationName = aws.ToString(tag.Value)
		}
	}

	appURL := utils.DefaultAppPublicAddr(strings.ToLower(applicationName), proxyPublicAddr)

	labels[types.AWSAccountIDLabel] = parsedProfileARN.AccountID
	labels[constants.AWSAccountIDLabel] = parsedProfileARN.AccountID
	labels[types.IntegrationLabel] = integrationName
	labels[types.AWSRolesAnywhereProfileARNLabel] = profileARN

	// TODO(marco): add origin label in v19: teleport.dev/origin: integration_awsrolesanywhere
	// types.Metadata.CheckAndSetDefaults in v17 returns an error if the origin label is set to AWS Roles Anywhere.
	// Only V18 application services support the origin label, so we can only add it in V19.

	expiration := params.Clock.Now().Add(params.SyncPollInterval * 2)

	appServer, err := types.NewAppServerV3(types.Metadata{
		Name:    applicationName,
		Labels:  labels,
		Expires: &expiration,
	}, types.AppServerSpecV3{
		HostID: params.HostUUID,
		App: &types.AppV3{
			Metadata: types.Metadata{
				Name:   applicationName,
				Labels: labels,
			},
			Spec: types.AppSpecV3{
				URI:         constants.AWSConsoleURL,
				Integration: integrationName,
				PublicAddr:  appURL,
				AWS: &types.AppAWS{
					RolesAnywhereProfile: &types.AppAWSRolesAnywhereProfile{
						ProfileARN:            aws.ToString(profile.ProfileArn),
						AcceptRoleSessionName: aws.ToBool(profile.AcceptRoleSessionName),
					},
				},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return appServer, nil
}
