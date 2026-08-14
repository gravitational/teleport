package okta

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/integrations/access/common"
)

type serviceStatusUpdater interface {
	UpdateUserSync(context.Context, time.Time /* nUsers */, int, error)
	UpdateAppGroupSync(context.Context, time.Time /* nApps*/, int /* nGroups */, int, error)
	UpdateAccessListSync(context.Context, time.Time /* nApps */, int /* nGroups */, int, error)
}

type serviceStatus struct {
	lock    sync.Mutex
	sink    common.StatusSink
	code    types.PluginStatusCode
	logger  *slog.Logger
	details *types.PluginOktaStatusV1
}

func (s *serviceStatus) UpdateUserSync(ctx context.Context, now time.Time, nUsers int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	userSync := s.details.UsersSyncDetails
	if err != nil {
		userSync.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		userSync.Error = formatError(err).Error()
		userSync.LastFailed = &now
	} else {
		userSync.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		userSync.Error = ""
		userSync.LastSuccessful = &now
		userSync.NumUsersSynced = int32(nUsers)
	}
	s.propagateErrors(err)
	ReportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) UpdateAppGroupSync(ctx context.Context, now time.Time, nApps, nGroups int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err != nil {
		s.details.AppGroupSyncDetails.StatusCode =
			types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		s.details.AppGroupSyncDetails.LastFailed = &now
		s.details.AppGroupSyncDetails.Error = formatError(err).Error()
	} else {
		s.details.AppGroupSyncDetails.StatusCode =
			types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		s.details.AppGroupSyncDetails.Error = ""
		s.details.AppGroupSyncDetails.NumAppsSynced = int32(nApps)
		s.details.AppGroupSyncDetails.NumGroupsSynced = int32(nGroups)
		s.details.AppGroupSyncDetails.LastSuccessful = &now
	}
	s.propagateErrors(err)
	ReportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) UpdateAccessListSync(ctx context.Context, now time.Time, nApps, nGroups int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	acl := s.details.AccessListsSyncDetails
	if err != nil {
		acl.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		acl.Error = formatError(err).Error()
		acl.LastFailed = &now
	} else {
		acl.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		acl.Error = ""
		acl.NumAppsSynced = int32(nApps)
		acl.NumGroupsSynced = int32(nGroups)
		acl.LastSuccessful = &now
	}
	s.propagateErrors(err)
	ReportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) UpdateSystemLogExporter(ctx context.Context, now time.Time, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	sle := s.details.SystemLogExportDetails
	if err != nil {
		sle.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		sle.Error = formatError(err).Error()
		sle.LastFailed = &now
	} else {
		sle.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		sle.Error = ""
		sle.LastSuccessful = &now
	}
	s.propagateErrors(err)
	ReportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) propagateErrors(err error) {
	const statusError = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR

	switch {
	case err == nil:
		if s.details.AppGroupSyncDetails.StatusCode != statusError &&
			s.details.UsersSyncDetails.StatusCode != statusError &&
			s.details.AccessListsSyncDetails.StatusCode != statusError &&
			s.details.SystemLogExportDetails.StatusCode != statusError {
			s.code = types.PluginStatusCode_RUNNING
		}

	case trace.IsAccessDenied(err):
		s.code = types.PluginStatusCode_UNAUTHORIZED

	default:
		s.code = types.PluginStatusCode_OTHER_ERROR
	}
}

const (
	deadlineExceededMessage = "Request timed out. Service may be rate limited."
)

func formatError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return trace.Wrap(err, deadlineExceededMessage)

	default:
		return err
	}
}

// ReportPluginStatus will report the plugin status to the given status sink if it exists.
func ReportPluginStatus(ctx context.Context, log *slog.Logger, pluginStatusSink common.StatusSink, code types.PluginStatusCode, details *types.PluginOktaStatusV1) {
	ReportPluginStatusError(ctx, log, pluginStatusSink, code, details, "")
}

func ReportPluginStatusError(ctx context.Context, log *slog.Logger, pluginStatusSink common.StatusSink, code types.PluginStatusCode, details *types.PluginOktaStatusV1, message string) {
	if pluginStatusSink == nil {
		return
	}

	if err := pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code:         code,
		Details:      &types.PluginStatusV1_Okta{Okta: details},
		ErrorMessage: message,
	}); err != nil {
		log.ErrorContext(ctx, "Error emitting plugin status", "error", err)
	}
}

type PluginOktaStatusParams struct {
	SsoConnector types.SAMLConnector
	SyncSettings types.PluginOktaSyncSettings
	ScimEnabled  bool
	SyncErr      error
}

func NewPluginOktaStatus(params PluginOktaStatusParams) *types.PluginOktaStatusV1 {
	var mappedRoleNames []string

	// We want to find any role names applied to the 'everyone' group for
	// the integration's SAML connector, and provide those to the frontend
	// for use on the Status page.
	if params.SsoConnector != nil && len(params.SsoConnector.GetAttributesToRoles()) > 0 {
		for _, mapping := range params.SsoConnector.GetAttributesToRoles() {
			if mapping.Value == oktaapi.OktaGroupEveryone {
				mappedRoleNames = append(mappedRoleNames, mapping.Roles...)
				break
			}
		}
	}

	syncErrorMsg := ""
	if params.SyncErr != nil {
		syncErrorMsg = params.SyncErr.Error()
	}

	return &types.PluginOktaStatusV1{
		SsoDetails: &types.PluginOktaStatusDetailsSSO{
			Enabled:                      true,
			AppId:                        params.SyncSettings.AppId,
			AppName:                      params.SyncSettings.AppName,
			OktaGroupEveryoneMappedRoles: mappedRoleNames,
		},
		ScimDetails: &types.PluginOktaStatusDetailsSCIM{
			Enabled: params.ScimEnabled,
		},
		UsersSyncDetails: &types.PluginOktaStatusDetailsUsersSync{
			Enabled: params.SyncSettings.GetEnableUserSync(),
			Error:   syncErrorMsg,
		},
		AppGroupSyncDetails: &types.PluginOktaStatusDetailsAppGroupSync{
			Enabled: params.SyncSettings.GetEnableAppGroupSync(),
			Error:   syncErrorMsg,
		},
		AccessListsSyncDetails: &types.PluginOktaStatusDetailsAccessListsSync{
			Enabled:      params.SyncSettings.GetEnableAccessListSync(),
			Error:        syncErrorMsg,
			GroupFilters: params.SyncSettings.GroupFilters,
			AppFilters:   params.SyncSettings.AppFilters,
		},
		SystemLogExportDetails: &types.PluginOktaStatusSystemLogExporter{
			Enabled: params.SyncSettings.GetEnableSystemLogExport(),
			Error:   syncErrorMsg,
		},
	}
}
