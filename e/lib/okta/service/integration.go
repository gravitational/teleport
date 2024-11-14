package oktaservice

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gravitational/trace"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/e/lib/okta/common/sso"
	"github.com/gravitational/teleport/e/lib/plugins"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func (s *Service) authorize(ctx context.Context, verb string) error {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindPlugin, verb); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ValidateClientCredentials validates the client credentials by making a request to the Okta API.
func (s *Service) ValidateClientCredentials(ctx context.Context, req *oktapb.ValidateClientCredentialsRequest) (*oktapb.ValidateClientCredentialsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	oktaClient, err := s.createOktaClient(ctx, req)
	if err != nil {
		return nil, trace.BadParameter("okta credential verification failed: %v", err)
	}
	if _, err = oktaClient.ListUsers(ctx); err != nil {
		return nil, trace.BadParameter("okta credential verification failed: %v", err)
	}
	return &oktapb.ValidateClientCredentialsResponse{}, nil
}

func (s *Service) GetGroups(ctx context.Context, req *oktapb.GetGroupsRequest) (*oktapb.GetGroupsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	groups, err := s.fetchAllOktaGroups(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := filterOktaResources(req.GetFilters(), groups)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.GetGroupsResponse{
		Groups: toGroups(filtered),
	}, nil
}

func (s *Service) GetApps(ctx context.Context, req *oktapb.GetAppsRequest) (*oktapb.GetAppsResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := s.fetchAllOktaApps(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	filtered, err := filterOktaResources(req.GetFilters(), apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.GetAppsResponse{
		Apps: toApps(filtered),
	}, nil
}

// CreateIntegration creates a new Okta integration.
// Depending on the request, it may create a new SAML connector or reuse an existing one.
func (s *Service) CreateIntegration(ctx context.Context, req *oktapb.CreateIntegrationRequest) (*oktapb.CreateIntegrationResponse, error) {
	if err := s.authorize(ctx, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}

	var oktaClient api.Client
	var err error
	if req.GetApiCredentials() != nil {
		oktaClient, err = s.createOktaClient(ctx, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	info, err := s.getOrCreateSAMLConnector(ctx, oktaClient, req.GetReuseConnector())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	oktaPlugin := newOktaPlugin(req, info)
	createPluginRequest := &pluginspb.CreatePluginRequest{
		Plugin:                oktaPlugin,
		StaticCredentialsList: getOktaPluginCredentials(req),
		CredentialLabels: map[string]string{
			eteleport.OktaOrgURLLabel: req.GetOktaOrganizationUrl(),
		},
	}
	if _, err = s.pluginService.CreatePlugin(ctx, createPluginRequest); err != nil {
		return nil, trace.Wrap(err)
	}
	return &oktapb.CreateIntegrationResponse{
		Plugin: oktaPlugin,
		ConnectorInfo: &oktapb.ConnectorInfo{
			OktaAppId:             info.OktaAppID,
			OktaAppName:           info.OktaAppName,
			OktaAppLabels:         info.OktaAppLabel,
			TeleportConnectorName: info.Connector.GetName(),
		},
	}, nil
}

func (s *Service) UpdateIntegration(ctx context.Context, req *oktapb.UpdateIntegrationRequest) (*oktapb.UpdateIntegrationResponse, error) {
	if err := s.authorize(ctx, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, trace.NotImplemented("not implemented")
}

func newOktaPlugin(req *oktapb.CreateIntegrationRequest, info *sso.SAMLConnectorInfo) *types.PluginV1 {
	oktaSettings := &types.PluginOktaSettings{
		OrgUrl: req.GetOktaOrganizationUrl(),
		SyncSettings: &types.PluginOktaSyncSettings{
			SyncUsers:            req.GetEnableUserSync(),
			SyncAccessLists:      req.GetEnableAccessListSync(),
			DisableSyncAppGroups: !req.GetEnableAppGroupSync(),
			SsoConnectorId:       info.Connector.GetName(),
			AppId:                info.OktaAppID,

			GroupFilters:  req.GetAccessListSettings().GetGroupFilters(),
			AppFilters:    req.GetAccessListSettings().GetAppFilters(),
			DefaultOwners: req.GetAccessListSettings().GetDefaultOwner(),
		},
	}
	return &types.PluginV1{
		SubKind: types.PluginSubkindAccess,
		Metadata: types.Metadata{
			Labels: map[string]string{
				plugins.HostedPluginLabel: "true",
			},
			Name: types.PluginTypeOkta,
		},
		Spec: types.PluginSpecV1{
			Settings: &types.PluginSpecV1_Okta{
				Okta: oktaSettings,
			},
		},
	}
}

type oktaAuthConfigGetter interface {
	// GetOktaOrganizationUrl returns the Okta organization URL.
	GetOktaOrganizationUrl() string
	// GetApiCredentials returns the Okta API credentials.
	GetApiCredentials() *oktapb.OktaAPICredentials
}

func (s *Service) createOktaClient(ctx context.Context, params oktaAuthConfigGetter) (api.Client, error) {
	if err := validateCredential(params); err != nil {
		return nil, trace.Wrap(err)
	}

	oktaClient, err := s.apiClientProviderFn(ctx, api.ClientConfig{
		HTTPClient:   &http.Client{Transport: s.roundTripper},
		Endpoint:     params.GetOktaOrganizationUrl(),
		AuthProvider: api.NewSSWSAuthProvider(params.GetApiCredentials().GetSswsBearerToken()),
		Log:          slog.With("okta_url", params.GetOktaOrganizationUrl()),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return oktaClient, nil
}
