package pluginsv1

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/auth"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	icutils "github.com/gravitational/teleport/lib/utils/aws/identitycenterutils"
)

// awsicPluginHandler defines a validation and update handler for the AWS
// Identity Center integration
type awsicPluginHandler struct{}

// validatePlugin implements [pluginHandler] for the [awsicPluginHandler].
func (awsicPluginHandler) validatePlugin(ctx context.Context, plugin *types.PluginV1, auth *auth.Server) error {
	settings := plugin.Spec.GetAwsIc()
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

	if err := validateAWSICCredentials(ctx, settings.Credentials, auth); err != nil {
		return trace.Wrap(err, "invalid credentials")
	}

	return nil
}

type integrationGetter interface {
	GetIntegration(context.Context, string) (types.Integration, error)
}

func validateAWSICCredentials(ctx context.Context, credentials *types.AWSICCredentials, services integrationGetter) error {
	switch creds := credentials.GetSource().(type) {
	case *types.AWSICCredentials_Oidc:
		// check that the presented integration exists
		if creds.Oidc == nil {
			return trace.BadParameter("missing inner OIDC credentials")
		}

		if _, err := services.GetIntegration(ctx, creds.Oidc.IntegrationName); err != nil {
			return trace.Wrap(err)
		}
		return nil

	default:
		// no other credential types need validation
		return nil
	}
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
