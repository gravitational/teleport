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
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/utils"
)

// AWSRolesAnywhereProfileSyncerParams contains the parameters for the AWS Roles Anywhere Profile Syncer.
type AWSRolesAnywhereProfileSyncerParams struct {
	// Clock is used to calculate the expiration time of the AppServers.
	Clock clockwork.Clock

	// Logger is used to log messages.
	Logger *slog.Logger

	// KeyStoreManager grants access to the AWS Roles Anywhere signer.
	KeyStoreManager KeyStoreManager

	// Cache is used to get the current cluster name and cert authority keys.
	Cache SyncerCache

	// Backend is used access the lock primitives to ensure only one Syncer is running at any given time.
	Backend backend.Backend

	// StatusReporter is used to report the status of the syncer.
	StatusReporter StatusReporter

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

// StatusReporter is an interface that defines methods for reporting the status of the syncer.
type StatusReporter interface {
	// UpdateIntegration updates the current integration status.
	UpdateIntegration(ctx context.Context, req types.Integration) (types.Integration, error)
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

func (p *AWSRolesAnywhereProfileSyncerParams) checkAndSetDefaults() error {
	if p.KeyStoreManager == nil {
		return trace.BadParameter("key store manager is required")
	}

	if p.Cache == nil {
		return trace.BadParameter("cache client is required")
	}

	if p.StatusReporter == nil {
		return trace.BadParameter("status reporter is required")
	}

	if p.AppServerUpserter == nil {
		return trace.BadParameter("app server upserter is required")
	}

	if p.Backend == nil {
		return trace.BadParameter("backend is required")
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

// RunAWSRolesAnywhereProfileSyncerWhileLocked runs the AWS Roles Anywhere Profile Syncer.
// It will iterate over all AWS IAM Roles Anywhere integrations, and for each one:
// 1. Check if the Profile Sync is enabled.
// 2. Generate AWS credentials using the integration.
// 3. List all profiles in the AWS IAM Roles Anywhere service.
// 4. For each profile, check if it is enabled and has associated roles.
// 5. Create an AppServer for each profile, using the profile name as the AppServer name.
// AppServer name can be overridden by the `TeleportApplicationName` tag on the Profile.
//
// This function will run the AWS Roles Anywhere Profile Syncer while holding a lock to ensure it doesn't race on multiple auth instances.
func RunAWSRolesAnywhereProfileSyncerWhileLocked(ctx context.Context, params AWSRolesAnywhereProfileSyncerParams) error {
	if err := params.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	runWhileLockedConfig := backend.RunWhileLockedConfig{
		LockConfiguration: backend.LockConfiguration{
			Backend:            params.Backend,
			LockNameComponents: []string{"aws-roles-anywhere.profile-sync"},
			TTL:                time.Minute,
			RetryInterval:      params.SyncPollInterval,
		},
		RefreshLockInterval: 20 * time.Second,
	}

	waitWithJitter := retryutils.SeventhJitter(time.Second * 10)
	for {
		err := backend.RunWhileLocked(ctx, runWhileLockedConfig, runProfileSyncer(params))
		if err != nil && ctx.Err() == nil {
			params.Logger.ErrorContext(
				ctx,
				"AWS Roles Anywhere profile syncer encountered a fatal error, it will restart after backoff",
				"error", err,
				"restart_after", waitWithJitter,
			)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(waitWithJitter):
		}
	}
}

func runProfileSyncer(params AWSRolesAnywhereProfileSyncerParams) func(context.Context) error {
	return func(ctx context.Context) error {
		for {
			if err := profileSyncIteration(ctx, params); err != nil {
				return trace.Wrap(err)
			}

			select {
			case <-ctx.Done():
				return nil

			case <-params.Clock.After(params.SyncPollInterval):
			}
		}
	}
}

func profileSyncIteration(ctx context.Context, params AWSRolesAnywhereProfileSyncerParams) error {
	integrations, err := integrationsWithProfileSyncEnabled(ctx, params.Cache)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(integrations) == 0 {
		return nil
	}

	proxyPublicAddr, err := fetchProxyPublicAddr(params.Cache)
	if err != nil {
		if trace.IsNotFound(err) {
			params.Logger.WarnContext(ctx, "AWS IAM Roles Anywhere Profile Syncer requires a Proxy which isn't available yet. It will retry again later.")
			return nil
		}

		return trace.Wrap(err)
	}

	for _, integration := range integrations {
		syncSummary := syncProfileForIntegration(ctx, params, integration, proxyPublicAddr)
		if syncSummary.setupError != nil {
			// Only log the error if there was a set up error (eg, invalid sync configuration, missing permissions, ...).
			// Profile specific errors (eg, invalid application url) were already logged.
			params.Logger.WarnContext(ctx, "failed to sync AWS Roles Anywhere Profiles for integration", "error", syncSummary.setupError)
		}

		integration = updateIntegrationStatus(integration, syncSummary)

		if _, err := params.StatusReporter.UpdateIntegration(ctx, integration); err != nil {
			params.Logger.ErrorContext(ctx, "failed to update integration status", "integration", integration.GetName(), "error", err)
		}
	}

	return nil
}

func updateIntegrationStatus(integration types.Integration, syncSummary *syncSummary) types.Integration {
	syncError := syncSummary.setupError
	if syncError == nil {
		syncError = trace.NewAggregate(syncSummary.profileErrors...)
	}

	status := types.IntegrationAWSRolesAnywhereProfileSyncStatusSuccess
	if syncError != nil {
		status = types.IntegrationAWSRolesAnywhereProfileSyncStatusError
	}

	integration.SetStatus(types.IntegrationStatusV1{
		AWSRolesAnywhere: &types.AWSRAIntegrationStatusV1{
			LastProfileSync: &types.AWSRolesAnywhereProfileSyncIterationSummary{
				StartTime:      syncSummary.startTime,
				EndTime:        syncSummary.endTime,
				Status:         status,
				SyncedProfiles: int32(syncSummary.syncedProfiles),
				ErrorMessage:   truncateErrorMessage(syncError),
			},
		},
	})

	return integration
}

func truncateErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	errorMessage := err.Error()

	if len(errorMessage) <= defaults.DefaultMaxErrorMessageSize {
		return errorMessage
	}

	return errorMessage[:defaults.DefaultMaxErrorMessageSize]
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

func buildAWSRolesAnywhereClientForIntegration(ctx context.Context, params AWSRolesAnywhereProfileSyncerParams, integration types.Integration) (RolesAnywhereClient, error) {
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

type syncSummary struct {
	startTime time.Time
	endTime   time.Time

	// set up error is the error that occurred while setting up the syncer
	// Examples:
	// - invalid Integration configuration which prevents creating the AWS SDK client (ie, invalid trust anchor ARN, profile ARN, or role ARN)
	// - failure to generate credentials to obtain the AWS SDK client
	// - failure to list IAM Roles Anywhere Profiles (eg, IAM Role has an invalid policy)
	setupError error

	// syncedProfiles is the number of profiles that were successfully synced.
	syncedProfiles int

	// Profile errors are errors that occurred while processing individual profiles.
	// Examples:
	// - failure in converting a Profile to an AppServer (eg, invalid URL)
	// - failure creating an AppServer from a Profile
	//
	// Always empty if setupError is not nil.
	profileErrors []error
}

func syncProfileForIntegration(ctx context.Context, params AWSRolesAnywhereProfileSyncerParams, integration types.Integration, proxyPublicAddr string) *syncSummary {
	logger := params.Logger.With("integration", integration.GetName())

	ret := &syncSummary{
		startTime: params.Clock.Now(),
	}

	defer func() {
		ret.endTime = params.Clock.Now()
	}()

	raClient, err := buildAWSRolesAnywhereClientForIntegration(ctx, params, integration)
	if err != nil {
		ret.setupError = trace.Wrap(err)
		return ret
	}

	profileNameFilters := integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.ProfileNameFilters
	profileUsedForProfileSync := integration.GetAWSRolesAnywhereIntegrationSpec().ProfileSyncConfig.ProfileARN

	var nextPage *string
	for {
		listReq := listRolesAnywhereProfilesRequest{
			nextPage:           nextPage,
			filters:            profileNameFilters,
			ignoredProfileARNs: []string{profileUsedForProfileSync},
		}
		profilesListResp, respNextToken, err := listRolesAnywhereProfilesPage(ctx, raClient, listReq)
		if err != nil {
			ret.setupError = trace.Wrap(err)
			return ret
		}

		for _, profile := range profilesListResp {
			err := processProfile(ctx, processProfileRequest{
				Params:          params,
				Profile:         profile,
				RAClient:        raClient,
				Integration:     integration,
				ProxyPublicAddr: proxyPublicAddr,
			})
			if err != nil {
				if errors.Is(err, errDisabledProfile) {
					logger.DebugContext(ctx, "Skipping profile", "profile_name", profile.Name, "error", err.Error())
					continue
				}

				logger.WarnContext(ctx, "Failed to process profile", "profile_name", profile.Name, "error", err)
				ret.profileErrors = append(ret.profileErrors, err)
				continue
			}

			ret.syncedProfiles++
		}

		if aws.ToString(respNextToken) == "" {
			break
		}
		nextPage = respNextToken
	}

	return ret
}

var (
	errDisabledProfile = errors.New("profile is disabled")
)

type processProfileRequest struct {
	Params          AWSRolesAnywhereProfileSyncerParams
	Profile         *integrationv1.RolesAnywhereProfile
	RAClient        RolesAnywhereClient
	Integration     types.Integration
	ProxyPublicAddr string
}

func processProfile(ctx context.Context, req processProfileRequest) error {
	if !req.Profile.Enabled {
		return errDisabledProfile
	}

	appServer, err := convertProfile(req.Params, req.Profile, req.Integration.GetName(), req.ProxyPublicAddr)
	if err != nil {
		return trace.BadParameter("failed to convert Profile to AppServer: %v", err)
	}

	if _, err := req.Params.AppServerUpserter.UpsertApplicationServer(ctx, appServer); err != nil {
		return trace.BadParameter("failed to upsert application server from Profile: %v", err)
	}

	return nil
}

func convertProfile(params AWSRolesAnywhereProfileSyncerParams, profile *integrationv1.RolesAnywhereProfile, integrationName string, proxyPublicAddr string) (types.AppServer, error) {
	parsedProfileARN, err := arn.Parse(profile.Arn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	applicationName := profile.Name + "-" + integrationName

	labels := make(map[string]string, len(profile.Tags))
	for tagKey, tagValue := range profile.Tags {
		labels["aws/"+tagKey] = tagValue

		if tagKey == types.AWSRolesAnywhereProfileNameOverrideLabel {
			applicationName = tagValue
		}
	}

	appURL := utils.DefaultAppPublicAddr(strings.ToLower(applicationName), proxyPublicAddr)

	labels[types.AWSAccountIDLabel] = parsedProfileARN.AccountID
	labels[constants.AWSAccountIDLabel] = parsedProfileARN.AccountID
	labels[types.IntegrationLabel] = integrationName
	labels[types.AWSRolesAnywhereProfileARNLabel] = profile.Arn

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
						ProfileARN:            profile.Arn,
						AcceptRoleSessionName: profile.AcceptRoleSessionName,
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
