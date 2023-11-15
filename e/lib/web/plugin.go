package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

const (
	pluginName = "web.enterprise"
)

// Config is a configuration of the web plugin
type Config struct {
	// Log is the logger
	Log logrus.FieldLogger

	// PluginShimURL is the URL for the Cloud plugin shim,
	// which is used to forward OAuth callbacks back to individual tenants
	// from a single domain.
	// If not set (i.e. in a single tenant or debug scenario),
	// the `redirect_uri` we submit to API providers will point
	// directly to the cluster, and the OAuth app needs to be configured accordingly.
	PluginShimURL *url.URL

	// AccessGraphClient is the access graph client.
	// Sets only when access graph is enabled.
	AccessGraphClient accessgraphv1.AccessGraphServiceClient
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(trace.Component, pluginName)
	}

	return nil
}

// NewPlugin creates an instance of the Enterprise Web Plugin
func NewPlugin(cfg Config) (*Plugin, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Plugin{
		Config: cfg,
	}, nil
}

// Plugin extends OSS auth server API with enterprise features
type Plugin struct {
	Config
	mu sync.RWMutex
	h  *web.Handler

	// samlIdP is the SAML identity provider.
	samlIdPMu sync.RWMutex
	samlIdP   *saml.Service

	// authMiddleware is the auth middleware.
	authMiddlewareMu sync.RWMutex
	authMiddleware   *auth.Middleware
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// GetProxyClient returns the proxy client.
func (p *Plugin) GetProxyClient() auth.ClientI {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.h.GetProxyClient()
}

// GetAccessPoint returns the proxy caching access point.
func (p *Plugin) GetAccessPoint() auth.ProxyAccessPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.h.GetAccessPoint()
}

// RegisterAuthServices registers GRPC services
func (p *Plugin) RegisterAuthServices(ctx context.Context, grpcServer interface{}) error {
	return nil
}

// RegisterAuthWebHandlers plugs in new handlers into OSS auth server router
func (p *Plugin) RegisterAuthWebHandlers(srv interface{}) error {
	return nil
}

// RegisterSAMLIdP will register the SAML IdP with the plugin.
//
//nolint:revive // Because we want this to be IdP.
func (p *Plugin) RegisterSAMLIdP(samlIdP *saml.Service) {
	p.samlIdPMu.Lock()
	defer p.samlIdPMu.Unlock()
	p.samlIdP = samlIdP
}

// RegisterProxyWebHandlers registers to proxy web handler
func (p *Plugin) RegisterProxyWebHandlers(handler interface{}) error {
	h, ok := handler.(*web.Handler)
	if !ok {
		return trace.BadParameter("unsupported handler type %T", handler)
	}

	p.mu.Lock()
	p.h = h
	p.mu.Unlock()

	clusterName, err := p.h.GetProxyClient().GetClusterName()
	if err != nil {
		return trace.Wrap(err)
	}

	p.authMiddlewareMu.Lock()
	p.authMiddleware = &auth.Middleware{
		ClusterName: clusterName.GetClusterName(),
	}
	p.authMiddlewareMu.Unlock()

	// Device Trust handlers
	h.GET("/enterprise/devices", h.WithAuth(p.listDevicesHandle))

	h.GET("/enterprise/authconnectors", h.WithAuth(p.getAuthConnectorsHandle))
	h.POST("/enterprise/saml", h.WithAuth(p.createSAMLConnectorHandle))
	h.PUT("/enterprise/saml/:name", h.WithAuth(p.updateSAMLConnectorHandle))
	h.DELETE("/enterprise/saml/:name", h.WithAuth(p.deleteSAMLConnectorHandle))

	h.POST("/enterprise/oidc", h.WithAuth(p.createOIDCConnectorHandle))
	h.PUT("/enterprise/oidc/:name", h.WithAuth(p.updateOIDCConnectorHandle))
	h.DELETE("/enterprise/oidc/:name", h.WithAuth(p.deleteOIDCConnectorHandle))

	// /webapi handlers have been moved from OSS Teleport, but the path must remain unchanged
	// for compatibility reasons (connector resources contain URLs with these paths)

	// SAML 2.0 callback handlers
	h.POST("/webapi/saml/acs", h.WithMetaRedirect(p.samlACSHandle))
	h.POST("/webapi/saml/acs/:connector", h.WithMetaRedirect(p.samlACSHandle))
	h.GET("/webapi/saml/sso", h.WithMetaRedirect(p.samlSSO))
	h.POST("/webapi/saml/login/console", h.WithLimiter(p.samlSSOConsole))

	// Endpoints for using Teleport as SAML IdP.
	h.POST("/enterprise/samlidp", h.WithAuth(p.upsertSAMLIdPServiceProviderHandle))

	// OIDC callback handlers
	h.GET("/webapi/oidc/login/web", h.WithRedirect(p.oidcLoginWeb))
	h.GET("/webapi/oidc/callback", h.WithMetaRedirect(p.oidcCallback))
	h.POST("/webapi/oidc/login/console", h.WithLimiter(p.oidcLoginConsole))

	h.GET("/enterprise/license/status", httplib.MakeHandler(p.getLicenseCheckStatusHandle))
	h.GET("/enterprise/license", h.WithAuth(p.getLicense))

	h.POST("/enterprise/accessrequest", h.WithClusterClientProvider(p.createAccessRequestHandle))
	h.PUT("/enterprise/accessrequest", h.WithClusterClientProvider(p.reviewAccessRequestHandle))
	h.DELETE("/enterprise/accessrequest/:requestId", h.WithAuth(p.deleteAccessRequestHandle))
	h.GET("/enterprise/accessrequest/:requestId", h.WithClusterClientProvider(p.getAccessRequestHandle))
	h.GET("/enterprise/accessrequest", h.WithClusterClientProvider(p.getAccessRequestsHandle))
	h.GET("/enterprise/resourcerequestroles", h.WithAuth(p.getResourceRequestRolesHandle))
	h.GET("/enterprise/accessrequest/:requestId/suggestions/accesslist", h.WithClusterClientProvider(p.getSuggestedAccessListsHandle))
	h.POST("/enterprise/accessrequest/:requestId/promote", h.WithClusterClientProvider(p.accessRequestPromoteHandle))

	h.GET("/enterprise/accesslist", h.WithAuth(p.getAccessLists))
	h.GET("/enterprise/accesslist/:accessListId", h.WithAuth(p.getAccessList))
	// use the same handler for create and update
	h.POST("/enterprise/accesslist", h.WithAuth(p.upsertAccessList))
	h.PUT("/enterprise/accesslist/:accessListId", h.WithAuth(p.upsertAccessList))
	h.DELETE("/enterprise/accesslist/:accessListId", h.WithAuth(p.deleteAccessList))
	h.POST("/enterprise/accesslist/:accessListId/members", h.WithAuth(p.addMembersToAccessList))
	h.POST("/enterprise/accesslist/:accessListId/reviews", h.WithAuth(p.reviewAccessList))
	// Deprecated: use /enterprise/accessrequest/:requestId/suggestions/accesslist instead.
	h.GET("/enterprise/accesslistsuggestions/accessrequest/:requestId", h.WithClusterClientProvider(p.getSuggestedAccessListsHandle))

	h.GET("/enterprise/releases", h.WithAuth(p.getReleases))

	// Plugins: RESTy endpoints (create/list/delete)
	// createPluginHandle expects html form request and
	//	-	For OAuth plugins: it responds with meta redirect. With meta redirect, browser takes user to
	//		OAuth provider for OAuth registration. OAuth plugins are created after successful callback from
	// 		OAuth provider with pluginCallbackHandle.
	//  -	For non-OAuth plugins: it creates plugin and responds with plugin status.
	h.POST("/enterprise/plugin", h.WithAuthCookieAndCSRF(p.createPluginHandle))
	// pluginCallbackHandle handles OAuth callback and creates plugin.
	h.GET("/enterprise/plugins/callback/:type", h.WithAuthCookieAndCSRF(p.pluginCallbackHandle))
	h.DELETE("/enterprise/plugin/:name", h.WithAuth(p.deletePluginHandle))
	// get enrolled plugins
	h.GET("/enterprise/plugin", h.WithAuth(p.getPluginsHandle))
	// get supported plugins
	h.GET("/enterprise/plugins/types", h.WithAuth(p.getAvailablePluginTypesHandle))

	// Security reports API
	h.GET("/webapi/sites/:site/audit/reports/:name", h.WithClusterAuth(p.getSecurityReport))
	h.DELETE("/webapi/sites/:site/audit/reports/:name", h.WithClusterAuth(p.deleteSecurityReport))
	h.GET("/webapi/sites/:site/audit/reports", h.WithClusterAuth(p.listSecurityReports))
	h.POST("/webapi/sites/:site/audit/reports", h.WithClusterAuth(p.upsertSecurityReport))
	h.GET("/webapi/sites/:site/audit/queries/:name", h.WithClusterAuth(p.getAuditQuery))
	h.DELETE("/webapi/sites/:site/audit/queries/:name", h.WithClusterAuth(p.deleteAuditQuery))
	h.GET("/webapi/sites/:site/audit/queries", h.WithClusterAuth(p.listAuditQueries))
	h.POST("/webapi/sites/:site/audit/queries", h.WithClusterAuth(p.upsertAuditQuery))
	h.GET("/webapi/sites/:site/audit/schema", h.WithClusterAuth(p.getSchema))
	h.POST("/webapi/sites/:site/audit/queries/run", h.WithClusterAuth(p.runAuditQuery))
	h.POST("/webapi/sites/:site/audit/queries/result", h.WithClusterAuth(p.getQueryResult))
	h.POST("/webapi/sites/:site/audit/reports/:name/run", h.WithClusterAuth(p.runSecurityReport))
	h.GET("/webapi/sites/:site/audit/reports/:name/result/days/:days", h.WithClusterAuth(p.getSecurityReportResult))
	h.GET("/webapi/sites/:site/audit/reports/:name/state/days/:days", h.WithClusterAuth(p.getSecurityReportState))

	h.POST("/webapi/sites/:site/integration/externalcloudaudit/generate", h.WithClusterAuth(externalCloudAuditGenerate))
	h.GET("/webapi/scripts/integration/externalcloudaudit-bootstrap.sh", h.WithLimiter(getExternalCloudAuditBootstrapScript))
	h.POST("/webapi/sites/:site/integration/externalcloudaudit/promote", h.WithClusterAuth(p.externalCloudAuditPromote))
	h.GET("/webapi/sites/:site/integration/externalcloudaudit/cluster", h.WithClusterAuth(p.externalCloudAuditGetCluster))
	h.GET("/webapi/sites/:site/integration/externalcloudaudit/draft", h.WithClusterAuth(p.externalCloudAuditGetDraft))
	h.DELETE("/webapi/sites/:site/integration/externalcloudaudit/cluster", h.WithClusterAuth(p.externalCloudAuditDeleteCluster))
	h.DELETE("/webapi/sites/:site/integration/externalcloudaudit/draft", h.WithClusterAuth(p.externalCloudAuditDeleteDraft))

	if p.h.ClusterFeatures.GetCloud() {
		h.DELETE("/enterprise/cloud/card", p.withCloudAuth(p.removeCardHandle))
		h.POST("/enterprise/cloud/card", p.withCloudAuth(p.addCardHandle))
		h.PUT("/enterprise/cloud/card", p.withCloudAuth(p.updateCardHandle))
		h.GET("/enterprise/cloud/billing", p.withCloudAuth(p.getBillingInformationHandle))
		h.DELETE("/enterprise/cloud/billing", p.withCloudAuth(p.cancelSubscriptionHandle))
		h.GET("/enterprise/cloud/billing-summary", p.withCloudAuth(p.getBillingSummaryInformationHandle))
		h.GET("/enterprise/cloud/nonbillable-summary", p.withCloudAuth(p.getNonBillableUsageSummaryHandle))
		h.GET("/enterprise/cloud/payments-invoices", p.withCloudAuth(p.getPaymentsInvoicesInformationHandle))
		h.GET("/enterprise/cloud/invoice-settings", p.withCloudAuth(p.getInvoiceSettingsInformationHandle))
		h.PUT("/enterprise/cloud/address", p.withCloudAuth(p.updateStripeAddressHandle))
		h.POST("/enterprise/cloud/setupintent", p.withCloudAuth(p.createSetupIntentHandle))
		h.PUT("/enterprise/cloud/billing-email", p.withCloudAuth(p.updateEmailHandle))
		h.PUT("/enterprise/cloud/billing-po", p.withCloudAuth(p.updatePurchaseOrderHandle))

		// Upgrade window related endpoints.
		h.GET("/enterprise/cloud/upgradewindowstart", p.withCloudAuth(p.getUpgradeWindowStartHourHandle))
		h.POST("/enterprise/cloud/upgradewindowstart", p.withCloudAuth(p.updateUpgradeWindowStartHourHandle))

		// surveyCompanyResponsesHandler gets survey company name responses for the account, no specific to the current user
		h.GET("/enterprise/cloud/survey/company", p.withCloud(p.surveyCompanyResponsesHandler))
		// surveyResultsHandler sends survey responses to sales center for persistence
		h.POST("/enterprise/cloud/survey", p.withCloudAuth(p.surveyResultsHandler))

		h.POST("/enterprise/cloud/teleportinvite", p.withCloudAuth(p.sendTeleportInviteHandle))
	}

	// Recovery related endpoints.
	if p.h.ClusterFeatures.GetRecoveryCodes() {
		p.Log.Infoln("enabling recovery endpoints")
		h.POST("/enterprise/cloud/recovery/start", p.withCloud(p.startAccountRecoveryHandle))
		h.POST("/enterprise/cloud/recovery/verify", p.withCloud(p.verifyAccountRecoveryHandle))
		h.POST("/enterprise/cloud/recovery/newcredentials", p.withCloud(p.completeAccountRecoveryHandle))
		h.GET("/enterprise/cloud/recovery/token/:token", p.withCloud(p.getAccountRecoveryTokenHandle))
		h.POST("/enterprise/cloud/recovery/codes", p.withCloud(p.createAccountRecoveryCodesHandle))
		h.GET("/enterprise/cloud/recovery/codes", h.WithAuth(p.getAccountRecoveryCodesMetadataHandle))
	}

	// Access graph
	h.GET("/enterprise/accessgraph/query", h.WithAuth(p.queryAccessGraph))
	h.GET("/enterprise/accessgraph/static/*file", h.WithUnauthenticatedHighLimiter(p.getAccessGraphFile))

	h.GET(fmt.Sprintf("%s/*unused", saml.IdPRoute), p.withSAMLAuth())
	h.POST(fmt.Sprintf("%s/*unused", saml.IdPRoute), p.withSAMLAuth())

	return nil
}

// CloudHandler is a authenticated handler that is used to provide an initialized instance of the cloud client API
type CloudHandler func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error)

type cloudPublicHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (interface{}, error)

// withCloudAuth authenticates and request and initializes an instance of the cloud client API
func (p *Plugin) withCloudAuth(fn CloudHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		ctx, err := p.h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cloudClient, err := cloud.NewClientFromConnection(ctx.GetClientConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return fn(w, r, ctx, cloudClient)
	})
}

func (p *Plugin) withSAMLAuth() httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		p.samlIdPMu.RLock()
		samlIdP := p.samlIdP
		p.samlIdPMu.RUnlock()

		if samlIdP == nil {
			p.Log.Debug("SAML IdP not set")
			return nil, trace.NotFound("SAML IdP not found")
		}

		// We need the middleware before we can continue
		p.authMiddlewareMu.RLock()
		authMiddleware := p.authMiddleware
		p.authMiddlewareMu.RUnlock()

		if authMiddleware == nil {
			return nil, trace.BadParameter("the middleware is not yet ready")
		}

		sessCtx, err := p.h.AuthenticateRequest(w, r, false)
		if err != nil {
			redirectURI := (&url.URL{
				Scheme:   "https",
				Host:     r.Host,
				Path:     r.URL.Path,
				RawQuery: url.QueryEscape(r.URL.Query().Encode()),
			}).String()
			http.Redirect(w, r, "/web/login?redirect_uri="+redirectURI, http.StatusSeeOther)
			return nil, nil
		}

		cert, err := sessCtx.GetX509Certificate()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		tlsConnState := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		}
		remoteAddr, err := utils.ParseAddr(r.RemoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		newCtx, err := authMiddleware.WrapContextWithUserFromTLSConnState(r.Context(), tlsConnState, remoteAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		samlIdP.ServeHTTP(w, r.WithContext(newCtx))
		return nil, nil
	})
}

// withCloud checks against CSRF attacks and provides an initiliazed instance of the cloud client API for public requests.
func (p *Plugin) withCloud(fn cloudPublicHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		if err := csrf.VerifyHTTPHeader(r); err != nil {
			p.Log.Warnf("unable to validate CSRF token %v", err)
			return nil, trace.AccessDenied("access denied")
		}

		client, err := p.getAuthClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		cloudClient, err := cloud.NewClientFromConnection(client.GetConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err := fn(w, r, params, cloudClient)
		if err != nil {
			// Hide 429 error.
			if trace.IsLimitExceeded(err) {
				p.Log.Warn(err)
				return nil, trace.AccessDenied("unable to process your request")
			}
			return nil, trace.Wrap(err)
		}

		return res, nil
	})
}

func (p *Plugin) getAuthClient() (*auth.Client, error) {
	proxyClient := p.h.GetProxyClient()

	// TODO(mcbattirola): Move away from type assertions to a more robust solution.
	switch c := proxyClient.(type) {
	case *auth.GithubConverter:
		authClient, ok := c.ClientI.(*auth.Client)
		if !ok {
			return nil, trace.BadParameter("unexpected underlying type for GithubConverter: %T", c.ClientI)
		}
		return authClient, nil
	case *auth.Client:
		return c, nil
	default:
		return nil, trace.BadParameter("unexpected underlying type for proxyClient: %T", proxyClient)
	}
}
