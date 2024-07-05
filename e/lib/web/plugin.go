package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/encoding/protojson"
	googleproto "google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/idp/saml"
	"github.com/gravitational/teleport/e/lib/okta"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/httplib/reverseproxy"
	"github.com/gravitational/teleport/lib/services"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"
)

const (
	pluginName = "web.enterprise"
)

type getCertFunc = func() (*tls.Certificate, error)

// AccessGraphConfig holds the configuration for AccessGraph if it's enabled in the proxy.
type AccessGraphConfig struct {
	// Addr is the Access Graph listener address
	Addr string
	// CA is the CA certificate in PEM format used to verify the Access Graph GRPC connection.
	CA []byte
	// Insecure is true if the Access Graph GRPC connection should be insecure.
	// Do not use in production.
	Insecure bool
	// CipherSuites is the list of cipher suites to use for the Access Graph connection.
	CipherSuites []uint16
}

// Config is a configuration of the web plugin
type Config struct {
	// Log is the logger
	Log *logrus.Entry

	// PluginShimURL is the URL for the Cloud plugin shim,
	// which is used to forward OAuth callbacks back to individual tenants
	// from a single domain.
	// If not set (i.e. in a single tenant or debug scenario),
	// the `redirect_uri` we submit to API providers will point
	// directly to the cluster, and the OAuth app needs to be configured accordingly.
	PluginShimURL *url.URL

	// AccessGraph is the configuration for AccessGraph if it's enabled in the proxy.
	AccessGraph *AccessGraphConfig

	// Clock is the clock used by the plugin.
	Clock clockwork.Clock
}

// CheckAndSetDefaults checks and sets the defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Log == nil {
		c.Log = logrus.WithField(teleport.ComponentKey, pluginName)
	}

	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}

	return nil
}

// NewPlugin creates an instance of the Enterprise Web Plugin
func NewPlugin(cfg Config) (*Plugin, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	oktaPluginConfigHelper, err := okta.NewPluginConfigHelper(okta.PluginConfigHelperConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Plugin{
		Config:                 cfg,
		oktaPluginConfigHelper: oktaPluginConfigHelper,
		pluginDescriptors:      maps.Clone(defaultPluginDescriptors),
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

	// oktaPluginConfigHelper helps with the multistage configuration of Okta plugins.
	oktaPluginConfigHelper *okta.PluginConfigHelper

	pluginDescriptors map[types.PluginType]pluginDescriptor

	accessGraphForwarder   *reverseproxy.Forwarder
	accessGraphForwarderMu sync.RWMutex
}

// GetName returns plugin name
func (p *Plugin) GetName() string {
	return pluginName
}

// GetProxyClient returns the proxy client.
func (p *Plugin) GetProxyClient() authclient.ClientI {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.h.GetProxyClient()
}

// GetAccessPoint returns the proxy caching access point.
func (p *Plugin) GetAccessPoint() authclient.ProxyAccessPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.h.GetAccessPoint()
}

// GetHighLimiter returns the WithHighLimiter from lib/web.
func (p *Plugin) GetHighLimiter() func(fn httplib.HandlerFunc) httprouter.Handle {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.h.WithHighLimiter
}

// RegisterAuthServices registers GRPC services
func (p *Plugin) RegisterAuthServices(ctx context.Context, grpcServer any, getClientCert getCertFunc) error {
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

	// If the proxy doesn't have the access_graph section enabled
	// on the configuration, we fall back to the
	// cluster's access graph config provided by the auth server.
	if p.Config.AccessGraph == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rsp, err := p.h.GetProxyClient().GetClusterAccessGraphConfig(ctx)
		cancel()
		switch {
		case trace.IsNotImplemented(err):
			p.Log.Debugf("Auth server does not implement access graph's GetClusterAccessGraphConfig")
		case err != nil:
			p.Log.WithError(err).Errorf("Failed to get access graph config from the Auth server")
			return trace.Wrap(err)
		default:
			if rsp.GetEnabled() {
				p.Config.AccessGraph = &AccessGraphConfig{
					Addr:     rsp.GetAddress(),
					CA:       rsp.GetCa(),
					Insecure: rsp.GetInsecure(),
				}
			}
		}

	}

	if p.Config.AccessGraph != nil {
		// If proxy is configured to use AccessGraph, check if it supports HTTP.
		// If it does, build the forwarder now. Otherwise, periodically check if/when it does.
		if err := p.checkAndBuildAccessGraphHTTPTransport(); err != nil {
			return trace.Wrap(err)
		}
	}

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

	h.POST("/webapi/saml/slo", h.WithMetaRedirect(p.samlSLOHandle))

	// Teleport SAML IdP API
	h.GET("/enterprise/samlidp/:name", h.WithAuth(p.getSAMLIdPServiceProviderHandle))
	h.POST("/enterprise/samlidp", h.WithAuth(p.createSAMLIdPServiceProviderHandle))
	h.PUT("/enterprise/samlidp/:name", h.WithAuth(p.updateSAMLIdPServiceProviderHandle))
	h.DELETE("/enterprise/samlidp/:name", h.WithAuth(p.deleteSAMLIdPServiceProviderHandle))

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

	// Crown Jewels
	h.POST("/enterprise/crownjewels", h.WithAuth(p.markCrownJewel))
	h.DELETE("/enterprise/crownjewels/:name", h.WithAuth(p.deleteCrownJewel))

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
	// validate (possibly partial) plugin config without trying to create the plugin itself
	h.POST("/enterprise/plugins/validate", h.WithAuth(p.validatePluginConfig))
	h.GET("/enterprise/plugins/needscleanup/:type", h.WithAuth(p.pluginNeedsCleanup))
	h.PUT("/enterprise/plugins/cleanup/:type", h.WithAuth(p.pluginCleanup))
	h.POST("/enterprise/pluginconfig/okta/groups", h.WithAuth(p.getOktaGroups))
	h.POST("/enterprise/pluginconfig/okta/apps", h.WithAuth(p.getOktaApps))

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

	// Access monitoring rule API
	h.GET("/webapi/sites/:site/accessmonitoringrule", h.WithClusterAuth(p.getAccessMonitoringRules))
	h.POST("/webapi/sites/:site/accessmonitoringrule", h.WithClusterAuth(p.createAccessMonitoringRule))
	h.PUT("/webapi/sites/:site/accessmonitoringrule/:name", h.WithClusterAuth(p.updateAccessMonitoringRule))
	h.DELETE("/webapi/sites/:site/accessmonitoringrule/:name", h.WithClusterAuth(p.deleteAccessMonitoringRule))

	h.POST("/webapi/sites/:site/integration/externalauditstorage/generate", h.WithClusterAuth(externalAuditStorageGenerate))
	h.GET("/webapi/scripts/integration/externalauditstorage-bootstrap.sh", h.WithLimiter(getExternalAuditStorageBootstrapScript))
	h.POST("/webapi/sites/:site/integration/externalauditstorage/promote", h.WithClusterAuth(p.externalAuditStoragePromote))
	h.GET("/webapi/sites/:site/integration/externalauditstorage/cluster", h.WithClusterAuth(p.externalAuditStorageGetCluster))
	h.GET("/webapi/sites/:site/integration/externalauditstorage/draft", h.WithClusterAuth(p.externalAuditStorageGetDraft))
	h.DELETE("/webapi/sites/:site/integration/externalauditstorage/cluster", h.WithClusterAuth(p.externalAuditStorageDeleteCluster))
	h.DELETE("/webapi/sites/:site/integration/externalauditstorage/draft", h.WithClusterAuth(p.externalAuditStorageDeleteDraft))

	// the billing summary API is available for cloud users
	// as well as self-hosted dashboards for usage-based customers
	isDashboard := services.IsDashboard(p.h.ClusterFeatures)
	isUsageBasedEnterprise := p.h.ClusterFeatures.GetProductType() == proto.ProductType_PRODUCT_TYPE_EUB
	if p.h.ClusterFeatures.GetCloud() || (isDashboard && isUsageBasedEnterprise) {
		h.GET("/enterprise/cloud/billing-summary", p.withCloudAuth(p.getBillingSummaryInformationHandle))
	}

	if p.h.ClusterFeatures.GetCloud() {
		h.DELETE("/enterprise/cloud/card", p.withCloudAuth(p.removeCardHandle))
		h.POST("/enterprise/cloud/card", p.withCloudAuth(p.addCardHandle))
		h.PUT("/enterprise/cloud/card", p.withCloudAuth(p.updateCardHandle))
		h.GET("/enterprise/cloud/billing", p.withCloudAuth(p.getBillingInformationHandle))
		h.DELETE("/enterprise/cloud/billing", p.withCloudAuth(p.cancelSubscriptionHandle))
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
		h.POST("/enterprise/cloud/teleportcredentialreset", p.withCloudAuth(p.sendTeleportCredentialResetHandle))
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
	h.GET("/enterprise/accessgraph/*path", p.accessGraphHandler(h))

	h.GET(fmt.Sprintf("%s/*unused", saml.IdPRoute), p.withSAMLAuth())
	h.POST(fmt.Sprintf("%s/*unused", saml.IdPRoute), p.withSAMLAuth())

	p.registerSCIMHandlers()

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

		res, err := fn(w, r, ctx, cloudClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// if the handler being called as fn returns a protobuf type,
		// encode it using protojson.
		// Otherwise, return the response directly and let our middleware
		// encode it with encoding/json.
		pm, ok := res.(googleproto.Message)
		if ok {
			result, err := protojson.Marshal(pm)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(result)

			return nil, nil
		}

		return res, trace.Wrap(err)
	})
}

// withSAMLAuth authenticates request against a valid Teleport web session except for
// the SAML IdP metadata endpoint "/saml-idp/metadata", which is served unauthenticated.
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

		// skip authenticating request for metadata endpoint
		if r.URL.Path == "/enterprise/saml-idp/metadata" || r.URL.Path == "/enterprise/saml-idp/metadata-values" {
			samlIdP.ServeHTTP(w, r)
			return nil, nil
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

func (p *Plugin) getAuthClient() (*authclient.Client, error) {
	proxyClient := p.h.GetProxyClient()

	// TODO(mcbattirola): Move away from type assertions to a more robust solution.
	switch c := proxyClient.(type) {
	case *auth.GithubConverter:
		authClient, ok := c.ClientI.(*authclient.Client)
		if !ok {
			return nil, trace.BadParameter("unexpected underlying type for GithubConverter: %T", c.ClientI)
		}
		return authClient, nil
	case *authclient.Client:
		return c, nil
	default:
		return nil, trace.BadParameter("unexpected underlying type for proxyClient: %T", proxyClient)
	}
}

func buildAccessGraphForwarder(tlsConfig *tls.Config) (*reverseproxy.Forwarder, error) {
	tr, err := defaults.Transport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = http2.ConfigureTransport(tr); err != nil {
		return nil, trace.Wrap(err)
	}
	tr.TLSClientConfig = tlsConfig

	// Don't trust any "X-Forward-*" headers the client sends, instead set our own.
	delegate := reverseproxy.NewHeaderRewriter()
	delegate.TrustForwardHeader = false

	accessGraphForwarder, err := reverseproxy.New(
		reverseproxy.WithRoundTripper(tr),
		reverseproxy.WithRewriter(common.NewHeaderRewriter(delegate, &teleportVersionHeaderAppender{})),
	)

	return accessGraphForwarder, trace.Wrap(err)
}

// checkAndBuildAccessGraphHTTPTransport checks if the access graph supports
// HTTP and builds the forwarder if it does.
// It builds the TLS config using the proxy identity and the cipher suites
// specified in the configuration.
func (p *Plugin) checkAndBuildAccessGraphHTTPTransport() error {
	tlsConfig, err := p.getProxyTLSConfig(p.Config.AccessGraph.CipherSuites)
	if err != nil {
		return trace.Wrap(err)
	}
	tlsConfig, err = getAccessGraphTLSConfig(p.Config.AccessGraph, tlsConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	// If the access graph supports HTTP, build the forwarder now.
	if p.accessGraphSupportsHTTP() {
		p.accessGraphForwarder, err = buildAccessGraphForwarder(tlsConfig)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		// Otherwise, periodically check if the access graph supports HTTP.
		// If it does, build the forwarder and replace the existing one.
		// Otherwise we will keep sending gRPC requests.
		go func() {
			retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
				First:  defaults.HighResPollingPeriod,
				Driver: retryutils.NewExponentialDriver(defaults.HighResPollingPeriod),
				Max:    defaults.LowResPollingPeriod,
				Jitter: retryutils.NewHalfJitter(),
				Clock:  p.Config.Clock,
			})
			if err != nil {
				p.Log.WithError(err).Debugf("Failed to create retry")
				return
			}

			// Periodically check if the access graph supports HTTP.
			for range retry.After() {
				retry.Inc()
				if p.accessGraphSupportsHTTP() {
					accessGraphForwarder, err := buildAccessGraphForwarder(tlsConfig)
					if err != nil {
						p.Log.Warnf("Failed to build access graph forwarder: %v", err)
						continue
					}
					p.accessGraphForwarderMu.Lock()
					p.accessGraphForwarder = accessGraphForwarder
					p.accessGraphForwarderMu.Unlock()
					return
				}
			}
		}()
	}
	return nil
}

// accessGraphSupportsHTTP returns true if the access graph service supports HTTP.
// This is determined by the presence of the grv_teleport_access_graph_http_enabled feature flag.
// This function uses the gRPC client through Auth to figure out if the feature is enabled.
func (p *Plugin) accessGraphSupportsHTTP() bool {
	client, err := p.getAuthClient()
	if err != nil {
		p.Log.WithError(err).Debugf("Failed to get auth client")
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// featuresFile is the file that contains the access graph features.
	const featuresFile = "features.json"
	rsp, err := client.AccessGraphClient().GetFile(
		ctx,
		&accessgraphv1.GetFileRequest{
			Filepath: featuresFile,
		},
	)
	if err != nil {
		p.Log.WithError(err).Debugf("Failed to get access graph features")
		return false
	}

	type jsonData struct {
		HTTPEnabled bool `json:"grv_teleport_access_graph_http_enabled"`
	}

	data := &jsonData{}
	if err := json.Unmarshal(rsp.Data, data); err != nil {
		p.Log.WithError(err).Debugf("Failed to parse access graph features payload")
		return false
	}
	// If the access graph does not support HTTP, return false.
	if !data.HTTPEnabled {
		p.Log.Debugf("Access graph does not support HTTP")
		return false
	}

	// now that we know the access graph supports HTTP, check if it's reachable.
	tlsConf, err := p.getProxyTLSConfig(nil)
	if err != nil {
		p.Log.WithError(err).Debugf("Failed to get proxy TLS config")
		return false
	}
	tlsConf, err = getAccessGraphTLSConfig(p.Config.AccessGraph, tlsConf)
	if err != nil {
		p.Log.WithError(err).Debugf("Failed to get access graph TLS config")
		return false
	}
	tr := &http.Transport{
		TLSClientConfig: tlsConf,
	}
	if err := http2.ConfigureTransport(tr); err != nil {
		p.Log.WithError(err).Debugf("Failed to configure transport")
		return false
	}
	httpClient := &http.Client{
		Transport: tr,
	}

	// Check if the access graph static assets are reachable.
	u := &url.URL{
		Scheme: "https",
		Host:   p.Config.AccessGraph.Addr,
		Path:   "/static/" + featuresFile,
	}
	// Use a context with a timeout to make the request.
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		p.Log.WithError(err).Debugf("Failed to create request")
		return false
	}
	httpRsp, err := httpClient.Do(req)
	if err != nil {
		p.Log.WithError(err).Warnf("Failed to make request. Please ensure proxy can reach access graph service at %v.", p.Config.AccessGraph.Addr)
		return false
	}
	defer httpRsp.Body.Close()
	io.Copy(io.Discard, httpRsp.Body)
	if httpRsp.StatusCode != http.StatusOK {
		p.Log.Warnf("Access graph static assets are not reachable. Please ensure proxy can reach access graph service at %v.", p.Config.AccessGraph.Addr)

	}
	return httpRsp.StatusCode == http.StatusOK
}

func (p *Plugin) getProxyTLSConfig(cipherSuites []uint16) (*tls.Config, error) {
	if testing.Testing() {
		return &tls.Config{
			InsecureSkipVerify: true,
		}, nil
	}
	tlsConfig, err := p.h.GetProxyClientTLSConfig(cipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tlsConfig, nil
}

// getAccessGraphTLSConfig builds the TLS config used to retrieve static assets from access graph service.
func getAccessGraphTLSConfig(cfg *AccessGraphConfig, tlsConfig *tls.Config) (*tls.Config, error) {
	const (
		// accessGraph expects this ALPN to redirect the request to the static HTTP server instead of the default
		// gRPC server.
		accessGraphStaticFileServerALPN = "http@server"
	)
	var caPool *x509.CertPool
	if len(cfg.CA) > 0 {
		caPool = x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(cfg.CA) {
			return nil, trace.BadParameter("unable to parse certificate")
		}
	}

	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	}

	tlsConfig.NextProtos = []string{accessGraphStaticFileServerALPN, string(alpncommon.ProtocolHTTP2), string(alpncommon.ProtocolHTTP)}
	tlsConfig.InsecureSkipVerify = cfg.Insecure
	tlsConfig.RootCAs = caPool
	tlsConfig.ServerName = "" /* empty server name to avoid SNI */
	return tlsConfig, nil
}

// teleportVersionHeaderAppender sets the X-TELEPORT-VERSION header on the request.
type teleportVersionHeaderAppender struct{}

// Rewrite request headers.
func (rw *teleportVersionHeaderAppender) Rewrite(req *httputil.ProxyRequest) {
	req.Out.Header.Set(teleport.VersionRequest, teleport.Version)
}
