package pluginsv1

import (
	"slices"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	cloudaws "github.com/gravitational/teleport/e/lib/cloud/aws"
	icfilters "github.com/gravitational/teleport/lib/aws/identitycenter/filters"
	icutils "github.com/gravitational/teleport/lib/utils/aws/identitycenterutils"
)

// awsicPluginHandler defines a validation and update handler for the AWS
// Identity Center integration
type awsicPluginHandler struct{}

// validatePlugin implements [pluginHandler] for the [awsicPluginHandler].
func (awsicPluginHandler) validatePlugin(plugin *types.PluginV1) error {
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

	return nil
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
	return a.Include.Equal(b.Include)
}
