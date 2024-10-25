package okta

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
)

type serviceStatusUpdater interface {
	SetCode(context.Context, types.PluginStatusCode)
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

func (s *serviceStatus) SetCode(ctx context.Context, code types.PluginStatusCode) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.code != code {
		s.code = code
		reportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
	}
}

func (s *serviceStatus) UpdateUserSync(ctx context.Context, now time.Time, nUsers int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	userSync := s.details.UsersSyncDetails
	if err != nil {
		userSync.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		userSync.Error = err.Error()
		userSync.LastFailed = &now
	} else {
		userSync.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		userSync.Error = ""
		userSync.LastSuccessful = &now
		userSync.NumUsersSynced = int32(nUsers)
	}
	reportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) UpdateAppGroupSync(ctx context.Context, now time.Time, nApps, nGroups int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err != nil {
		s.details.AppGroupSyncDetails.StatusCode =
			types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		s.details.AppGroupSyncDetails.LastFailed = &now
		s.details.AppGroupSyncDetails.Error = err.Error()
	} else {
		s.details.AppGroupSyncDetails.StatusCode =
			types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		s.details.AppGroupSyncDetails.Error = ""
		s.details.AppGroupSyncDetails.NumAppsSynced = int32(nApps)
		s.details.AppGroupSyncDetails.NumGroupsSynced = int32(nGroups)
		s.details.AppGroupSyncDetails.LastSuccessful = &now
	}

	reportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

func (s *serviceStatus) UpdateAccessListSync(ctx context.Context, now time.Time, nApps, nGroups int, err error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	acl := s.details.AccessListsSyncDetails
	if err != nil {
		acl.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_ERROR
		acl.Error = err.Error()
		acl.LastFailed = &now
	} else {
		acl.StatusCode = types.OktaPluginSyncStatusCode_OKTA_PLUGIN_SYNC_STATUS_CODE_SUCCESS
		acl.Error = ""
		acl.NumAppsSynced = int32(nApps)
		acl.NumGroupsSynced = int32(nGroups)
		acl.LastSuccessful = &now
	}

	reportPluginStatus(ctx, s.logger, s.sink, s.code, s.details)
}

// reportPluginStatus will report the plugin status to the given status sink if it exists.
func reportPluginStatus(ctx context.Context, log *slog.Logger, pluginStatusSink common.StatusSink, code types.PluginStatusCode, details *types.PluginOktaStatusV1) {
	if pluginStatusSink == nil {
		return
	}

	if err := pluginStatusSink.Emit(ctx, &types.PluginStatusV1{
		Code:    code,
		Details: &types.PluginStatusV1_Okta{Okta: details},
	}); err != nil {
		log.ErrorContext(ctx, "Error emitting plugin status", "error", err)
	}
}
