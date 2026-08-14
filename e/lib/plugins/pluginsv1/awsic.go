package pluginsv1

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssdkconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/e/lib/aws/identitycenter"
	icsdk "github.com/gravitational/teleport/e/lib/aws/identitycenter/sdk"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/lib/auth"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	awsconfig "github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/credprovider"
	"github.com/gravitational/teleport/lib/modules"
	icutils "github.com/gravitational/teleport/lib/utils/aws/identitycenterutils"
)

const (
	awsicAWSOIDCIntegrationDocsURL   = "https://goteleport.com/docs/enroll-resources/application-access/cloud-apis/awsoidc-integration/"
	awsicSystemCredentialsCloudError = `AWS Identity Center integrations on Teleport Cloud must authenticate to AWS with an AWS OIDC integration. When using tctl, rerun the command with --no-use-system-credentials --oidc-integration=<name>.`
)

// awsicPluginHandler defines a validation and update handler for the AWS
// Identity Center integration.
type awsicPluginHandler struct {
	modules                 modules.Modules
	newIdentityCenterClient func(icsdk.Config) (icsdk.Client, error)
	newSCIMClient           func(*scimsdk.Config) (scimsdk.Client, error)
}

const (
	awsICResourceSyncValidationError = "invalid AWS resource sync credentials"
	awsICSCIMValidationError         = "invalid SCIM credentials"
)

// validatePlugin implements [pluginHandler] for the [awsicPluginHandler].
func (h awsicPluginHandler) validatePlugin(ctx context.Context, input pluginValidationInput, auth *auth.Server) error {
	settings := input.plugin.Spec.GetAwsIc()
	if settings == nil {
		return trace.BadParameter("missing AWS IC settings")
	}

	if err := settings.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := cloudaws.ValidateAWSRegion(settings.Region); err != nil {
		return trace.Wrap(err)
	}

	if url, err := icutils.EnsureSCIMEndpoint(settings.ProvisioningSpec.BaseUrl); err != nil {
		return trace.Wrap(err)
	} else {
		settings.ProvisioningSpec.BaseUrl = url
	}

	if _, err := icfilters.New(settings.GroupSyncFilters); err != nil {
		return trace.Wrap(err, "malformed Group Sync Filters")
	}

	if _, err := icfilters.New(settings.AwsAccountsFilters); err != nil {
		return trace.Wrap(err, "malformed Account Sync Filters")
	}

	// AWSICRolesSyncModeNone stops the IC integration from creating roles for each
	// potential AWS Account Assignment. The AWS group importer expects these roles
	// to exist when creating Access Lists so that it can preserve the group account
	// assignments in Teleport. In order to avoid creating Access Lists that refer
	// to non-existent roles we also need to avoid importing AWS groups, so we enforce
	// a single exclude-everything GroupSyncFilter when RolesSyncMode is AWSICRolesSyncModeNone
	if settings.RolesSyncMode == types.AWSICRolesSyncModeNone {
		if !filterIsExcludesAll(settings.GroupSyncFilters) {
			return trace.BadParameter("roleSyncMode NONE requires group_sync_filters: [excludeNameRegex: *]")
		}
	}

	if h.modules.Features().Cloud && settings.Credentials.GetSystem() != nil {
		return trace.BadParameter("%s", awsicSystemCredentialsCloudErrorMessage(ctx, awsicIntegrationListerFromContext(ctx)))
	}

	return trace.Wrap(h.validateAWSICCredentials(ctx, settings, input, auth))
}

func (h awsicPluginHandler) validateAWSICCredentials(ctx context.Context, settings *types.PluginAWSICSettings, input pluginValidationInput, auth *auth.Server) error {
	// Only install-time validation should probe AWS/SCIM credentials via downstream APIs.
	// Plugin updates preserve credentials and should be allowed to skip.
	if !input.validateInstallCredentials {
		return nil
	}

	if err := h.validateAwsIcSDKCredentials(ctx, settings, auth, input.httpClient); err != nil {
		return trace.Wrap(err, awsICResourceSyncValidationError)
	}

	if err := h.validateAWSICSCIMCredentials(ctx, settings, input.staticCredentials, input.httpClient); err != nil {
		return trace.Wrap(err, awsICSCIMValidationError)
	}

	return nil
}

type integrationLister interface {
	ListIntegrations(context.Context, int, string) ([]types.Integration, string, error)
}

type awsicIntegrationListerContextKey struct{}

func withAWSICIntegrationLister(ctx context.Context, lister integrationLister) context.Context {
	return context.WithValue(ctx, awsicIntegrationListerContextKey{}, lister)
}

func awsicIntegrationListerFromContext(ctx context.Context) integrationLister {
	lister, _ := ctx.Value(awsicIntegrationListerContextKey{}).(integrationLister)
	return lister
}

func awsicSystemCredentialsCloudErrorMessage(ctx context.Context, lister integrationLister) string {
	parts := []string{awsicSystemCredentialsCloudError}

	if lister != nil {
		integrationNames, err := awsicAWSOIDCIntegrationNames(ctx, lister)
		if err != nil {
			slog.WarnContext(ctx, "Failed to list AWS OIDC integrations while building AWS Identity Center credentials error.", "error", err)
		}
		if err == nil && len(integrationNames) > 0 {
			parts = append(parts, "Existing AWS OIDC integrations: "+strings.Join(integrationNames, ", ")+".")
			return strings.Join(parts, " ")
		}
	}

	parts = append(parts, "To set up an AWS OIDC integration, see "+awsicAWSOIDCIntegrationDocsURL)
	return strings.Join(parts, " ")
}

func awsicAWSOIDCIntegrationNames(ctx context.Context, lister integrationLister) ([]string, error) {
	var integrationNames []string
	for integration, err := range clientutils.Resources(ctx, lister.ListIntegrations) {
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if integration.GetSubKind() == types.IntegrationSubKindAWSOIDC {
			integrationNames = append(integrationNames, integration.GetName())
		}
	}

	return integrationNames, nil
}

func (h awsicPluginHandler) validateAwsIcSDKCredentials(ctx context.Context, settings *types.PluginAWSICSettings, auth *auth.Server, httpClient *http.Client) error {
	switch creds := settings.Credentials.GetSource().(type) {
	case *types.AWSICCredentials_Oidc:
		// check that the presented integration exists
		if creds.Oidc == nil {
			return trace.BadParameter("missing inner OIDC credentials")
		}

		if _, err := auth.GetIntegration(ctx, creds.Oidc.IntegrationName); err != nil {
			return trace.Wrap(err)
		}

		cfg, err := cloudaws.CreateAWSConfigForIntegration(ctx, credprovider.Config{
			Region:                settings.Region,
			IntegrationName:       creds.Oidc.IntegrationName,
			IntegrationGetter:     auth.Services,
			AWSOIDCTokenGenerator: identitycenter.MakeTokenGenerator(auth),
			Clock:                 auth.GetClock(),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(h.validateAWSICResourceSyncCredential(ctx, settings.Arn, cfg))

	case *types.AWSICCredentials_System:
		if creds.System == nil {
			return trace.BadParameter("missing inner system credentials")
		}

		var (
			awsCfg aws.Config
			err    error
		)
		if creds.System.AssumeRoleArn != "" {
			awsCfg, err = cloudaws.BuildAWSConfig(ctx, settings.Region, creds.System.AssumeRoleArn, nil, awsHTTPClientOptions(httpClient)...)
		} else {
			awsCfg, err = awsconfig.LoadDefaultConfig(ctx, awsLoadOptions(settings.Region, httpClient)...)
		}
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(h.validateAWSICResourceSyncCredential(ctx, settings.Arn, &awsCfg))

	default:
		return nil
	}
}

func awsLoadOptions(region string, httpClient *http.Client) []func(*awssdkconfig.LoadOptions) error {
	options := []func(*awssdkconfig.LoadOptions) error{awssdkconfig.WithRegion(region)}
	return append(options, awsHTTPClientOptions(httpClient)...)
}

func awsHTTPClientOptions(httpClient *http.Client) []func(*awssdkconfig.LoadOptions) error {
	return []func(*awssdkconfig.LoadOptions) error{awssdkconfig.WithHTTPClient(httpClient)}
}

func (h awsicPluginHandler) validateAWSICResourceSyncCredential(ctx context.Context, instanceARN string, cfg *aws.Config) error {
	client, err := h.identityCenterClient(icsdk.Config{
		AWSConfig:   cfg,
		InstanceARN: instanceARN,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.ValidateResourceSyncCredential(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (h awsicPluginHandler) identityCenterClient(config icsdk.Config) (icsdk.Client, error) {
	if h.newIdentityCenterClient != nil {
		return h.newIdentityCenterClient(config)
	}
	return icsdk.New(config)
}

func (h awsicPluginHandler) validateAWSICSCIMCredentials(ctx context.Context, settings *types.PluginAWSICSettings, staticCredentials []*types.PluginStaticCredentialsV1, httpClient *http.Client) error {
	if len(staticCredentials) == 0 || staticCredentials[0] == nil || staticCredentials[0].Spec == nil {
		return trace.BadParameter("missing SCIM credentials")
	}

	bearerToken := staticCredentials[0].GetAPIToken()
	if bearerToken == "" {
		return trace.BadParameter("missing SCIM auth token")
	}

	scimClient, err := h.scimClient(&scimsdk.Config{
		HTTPClient:      httpClient,
		Endpoint:        settings.ProvisioningSpec.BaseUrl,
		Token:           bearerToken,
		IntegrationType: types.PluginTypeAWSIdentityCenter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := scimClient.Ping(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (h awsicPluginHandler) scimClient(config *scimsdk.Config) (scimsdk.Client, error) {
	if h.newSCIMClient != nil {
		return h.newSCIMClient(config)
	}
	return scimsdk.New(config)
}

var excludeAll types.AWSICResourceFilter_ExcludeNameRegex = types.AWSICResourceFilter_ExcludeNameRegex{ExcludeNameRegex: "*"}

func filterIsExcludesAll(filters []*types.AWSICResourceFilter) bool {
	if len(filters) != 1 {
		return false
	}

	f := filters[0]
	if f.Include != nil || f.Exclude == nil {
		return false
	}

	return excludeAll.Equal(f.Exclude)
}

// updatePlugin implements [awsicPluginHandler] for the [awsicPluginHandler].
//
// Checks for changes in the group import filter list, and if any changes are
// detected it will set the plugin group import status to REIMPORT_REQUESTED,
// which will trigger a new group import when the plugin is next restarted.
func (awsicPluginHandler) updatePlugin(newPlugin, oldPlugin *types.PluginV1) error {
	oldSettings := oldPlugin.Spec.GetAwsIc()
	newSettings := newPlugin.Spec.GetAwsIc()
	if oldSettings == nil || newSettings == nil {
		return trace.BadParameter("old and new plugins must both be AWS Identity Center integrations")
	}

	if newSettings.RolesSyncMode == types.AWSICRolesSyncModeNone &&
		oldSettings.RolesSyncMode != types.AWSICRolesSyncModeNone {
		return trace.BadParameter("invalid roles_sync_mode switch from %q to %q",
			oldSettings.RolesSyncMode, newSettings.RolesSyncMode)
	}

	oldStatus := oldPlugin.GetStatus()
	if !slices.EqualFunc(oldSettings.GroupSyncFilters, newSettings.GroupSyncFilters, icFiltersEq) {
		if oldStatus.GetAwsIc() == nil {
			oldStatus.SetDetails(&types.PluginStatusV1_AwsIc{AwsIc: &types.PluginAWSICStatusV1{GroupImportStatus: &types.AWSICGroupImportStatus{}}})
		}
		if oldStatus.GetAwsIc().GroupImportStatus == nil {
			oldStatus.GetAwsIc().GroupImportStatus = &types.AWSICGroupImportStatus{}
		}
		oldStatus.GetAwsIc().GroupImportStatus.StatusCode = types.AWSICGroupImportStatusCode_REIMPORT_REQUESTED
	}

	return trace.Wrap(newPlugin.SetStatus(oldStatus))
}

func icFiltersEq(a, b *types.AWSICResourceFilter) bool {
	return a.Equal(b)
}
