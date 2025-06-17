package web

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/encoding/protojson"
	googleproto "google.golang.org/protobuf/proto"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/trail"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/web"
)

const defaultCloudHTTPTimeout = 5 * time.Second

// CloudHandler is an authenticated handler that is used to provide an initialized instance of the cloud client API
type CloudHandler func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error)

type cloudPublicHandler func(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (any, error)

// ClusterCloudHandler is an authenticated handler that contains
// a cloudClient authenticated against a remoteSite as specified by the ":site" url parameter.
type ClusterCloudHandler func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error)

func (p *Plugin) registerCloudHandlers() {
	features := p.h.GetClusterFeatures()

	// the billing summary API is available for cloud users and usage-based self-hosted (dashboard) customers
	if features.GetCloud() || (services.IsDashboard(features) && features.IsUsageBased && !features.IsStripeManaged) {
		p.h.GET("/enterprise/cloud/billing-summary", p.withCloudAuth(p.withCloudCache(p.getBillingSummaryInformationHandle)))
	}

	// the following endpoints are only available to Cloud cloud-hosted customers (not Cloud dashboard customers)
	if features.GetCloud() {
		p.h.GET("/enterprise/cloud/billing", p.withCloudAuth(p.withCloudCache(p.getBillingInformationHandle)))
		p.h.GET("/enterprise/cloud/nonbillable-summary", p.withCloudAuth(p.withCloudCache(p.getNonBillableUsageSummaryHandle)))

		// Upgrade window related endpoints.
		// TODO(mcbattirola): remove on v19, since the endpoints are deprecated in favor of `enterprise/sites/:site/upgradewindowstart`.
		// Keeping it for now to ensure compatibility between proxies within one major of difference.
		p.h.GET("/enterprise/cloud/upgradewindowstart", p.withCloudAuth(p.withCloudCache(p.getUpgradeWindowStartHourHandle)))
		p.h.POST("/enterprise/cloud/upgradewindowstart", p.withCloudAuth(p.updateUpgradeWindowStartHourHandle))

		// surveyCompanyResponsesHandler gets survey company responses for the account.
		// Deprecated: no longer used.
		// TODO(bl-nero) DELETE IN v19.0.0
		p.h.GET("/enterprise/cloud/survey/company", p.withCloudAuth(p.withCloudCache(p.surveyCompanyResponsesHandler)))
	}

	// upgrade window endpoints with cluster param
	p.h.GET("/enterprise/sites/:site/upgradewindowstart", p.withCloudClusterAuth(p.withCloudClusterCache(p.getClusterUpgradeWindowStartHourHandle)))
	p.h.POST("/enterprise/sites/:site/upgradewindowstart", p.withCloudClusterAuth(p.updateClusterUpgradeWindowStartHourHandle))

	// contact endpoints
	p.h.GET("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.withCloudClusterCache(p.getClusterContactHandle)))
	p.h.POST("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.createClusterContactHandle))
	p.h.DELETE("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.deleteClusterContactHandle))
}

func (p *Plugin) getBillingSummaryInformationHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetBillingSummaryInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getBillingInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetBillingInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

// Deprecated: use `getClusterUpgradeWindowStartHourHandle` instead
// TODO(mcbattirola): remove in v18
func (p *Plugin) getUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetAccountUpgradeWindowStartHour(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

// Deprecated: use `updateClusterUpgradeWindowStartHourHandle` instead
// TODO(mcbattirola): remove in v18
func (p *Plugin) updateUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	var req cloudapi.UpdateAccountUpgradeWindowStartHourRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := client.UpdateAccountUpgradeWindowStartHour(r.Context(), &req); err != nil {
		return nil, trail.FromGRPC(err)
	}

	// emit audit event
	event := &apievents.UpgradeWindowStartUpdate{
		Metadata: apievents.Metadata{
			Type: events.UpgradeWindowStartUpdateEvent,
			Code: events.UpgradeWindowStartUpdatedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: ctx.GetUser(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: ctx.GetSessionID(),
		},
		UpgradeWindowStartMetadata: apievents.UpgradeWindowStartMetadata{
			UpgradeWindowStart: fmt.Sprintf("%02d:00:00", req.UpgradeWindowStartHour),
		},
	}

	if err := p.h.GetProxyClient().EmitAuditEvent(r.Context(), event); err != nil {
		p.Logger.WarnContext(r.Context(), "Failed to emit window upgrade start update event",
			"error", err,
			"user", event.UserMetadata.User,
			"upgrade_window_start", event.UpgradeWindowStartMetadata.UpgradeWindowStart,
		)
	}

	return web.OK(), nil
}

func (p *Plugin) getClusterUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
	res, err := cloudClient.GetAccountUpgradeWindowStartHour(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updateClusterUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
	var req cloudapi.UpdateAccountUpgradeWindowStartHourRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := cloudClient.UpdateAccountUpgradeWindowStartHour(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	// emit audit event
	event := &apievents.UpgradeWindowStartUpdate{
		Metadata: apievents.Metadata{
			Type: events.UpgradeWindowStartUpdateEvent,
			Code: events.UpgradeWindowStartUpdatedCode,
		},
		UserMetadata: apievents.UserMetadata{
			User: sctx.GetUser(),
		},
		SessionMetadata: apievents.SessionMetadata{
			SessionID: sctx.GetSessionID(),
		},
		UpgradeWindowStartMetadata: apievents.UpgradeWindowStartMetadata{
			UpgradeWindowStart: fmt.Sprintf("%02d:00:00", req.UpgradeWindowStartHour),
		},
	}

	if err := p.h.GetProxyClient().EmitAuditEvent(r.Context(), event); err != nil {
		p.Logger.WarnContext(r.Context(), "Failed to emit window upgrade start update event",
			"error", err,
			"user", event.UserMetadata.User,
			"upgrade_window_start", event.UpgradeWindowStartMetadata.UpgradeWindowStart,
		)
	}

	return web.OK(), nil
}

func (p *Plugin) surveyCompanyResponsesHandler(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetSurveyCompany(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) surveyResultsHandler(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	var req cloudapi.SetSurveyResultsRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := client.SetSurveyResults(r.Context(), &cloudapi.SetSurveyResultsRequest{
		CompanyName:   req.CompanyName,
		EmployeeCount: req.EmployeeCount,
		Resources:     req.Resources,
		Role:          req.Role,
		Team:          req.Team,
		Username:      ctx.GetUser(),
	})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return web.OK(), nil
}

func (p *Plugin) getClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
	contacts, err := cloudClient.GetContacts(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return contacts, nil
}

func (p *Plugin) createClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
	var req cloudapi.CreateContactRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := cloudClient.CreateContact(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return resp.Contact, nil
}

func (p *Plugin) deleteClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
	var req cloudapi.RemoveContactRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := cloudClient.RemoveContact(r.Context(), &req); err != nil {
		return nil, trail.FromGRPC(err)
	}
	return web.OK(), nil
}

// readProtoJSON reads a protojson-encoded request and unmarshals it
// into val.
func (p *Plugin) readProtoJSON(r *http.Request, val googleproto.Message) error {
	// Check content type to mitigate CSRF attack.
	// (Form POST requests don't support application/json payloads.)
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		p.Logger.WarnContext(r.Context(), "Error parsing media type for reading JSON", "error", err)
		return trace.BadParameter("invalid request")
	}

	if contentType != "application/json" {
		p.Logger.WarnContext(r.Context(), "Invalid HTTP request header content-type for reading JSON", "content_type", contentType)
		return trace.BadParameter("invalid request")
	}

	data, err := utils.ReadAtMost(r.Body, teleport.MaxHTTPRequestSize)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := (protojson.UnmarshalOptions{}).Unmarshal(data, val); err != nil {
		return trace.BadParameter("request: %v", err.Error())
	}
	return nil
}

// withCloudCache will store the last known successful response for `GET` requests and return it in cases when the API
// call fails or times out. This should be used when it is acceptable to return the last known value and errors are
// not expected during normal operation
func (p *Plugin) withCloudCache(fn CloudHandler) CloudHandler {
	var lastResult any
	var mu sync.Mutex

	return func(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
		result, err := fn(w, r, sctx, client)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, err
			}
			mu.Lock()
			defer mu.Unlock()
			if r.Method != http.MethodGet || lastResult == nil {
				return result, err
			}

			// return cache
			p.Logger.WarnContext(r.Context(), "Unable to get Cloud response, returning last cached value", "error", err, "url", logutils.StringerAttr(r.URL))
			return lastResult, nil
		}

		// no error, update cache & return
		if r.Method == http.MethodGet {
			mu.Lock()
			lastResult = result
			mu.Unlock()
		}
		return result, err
	}
}

// withCloudAuth authenticates and request and initializes an instance of the cloud client API
func (p *Plugin) withCloudAuth(next CloudHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
		sctx, err := p.h.AuthenticateRequest(w, r, true)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ctx, cancel := context.WithTimeout(r.Context(), defaultCloudHTTPTimeout)
		defer cancel()

		cloudClient, err := cloud.NewClientFromConnection(sctx.GetClientConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err := next(w, r.WithContext(ctx), sctx, cloudClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// if the handler being called as fn returns a protobuf type,
		// encode it using protojson.
		// Otherwise, return the response directly and let our middleware
		// encode it with encoding/json.
		pm, ok := res.(googleproto.Message)
		if ok {
			if err := writeProtoJSONResponse(w, pm); err != nil {
				return nil, trace.Wrap(err)
			}
			return nil, nil
		}

		return res, trace.Wrap(err)
	})
}

// withCloudClusterCache will store the last known successful response for `GET` requests and return it in cases when the API
// call fails or times out. This should be used when it is acceptable to return the last known value
func (p *Plugin) withCloudClusterCache(fn ClusterCloudHandler) ClusterCloudHandler {
	// this doesn't do any cleanup because we're not really expecting
	// more than one cluster, as this is a cloud-specific endpoint
	lastResults := make(map[string]any)
	var mu sync.Mutex

	return func(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, site reversetunnelclient.RemoteSite, cloudClient cloud.Client) (any, error) {
		result, err := fn(w, r, sctx, site, cloudClient)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, err
			}
			if r.Method == http.MethodGet {
				mu.Lock()
				defer mu.Unlock()
				if v := lastResults[site.GetName()]; v != nil {
					p.Logger.WarnContext(r.Context(), "Unable to get Cloud response, returning last cached value", "error", err, "url", r.URL.String())
					return v, nil
				}
			}
			return result, err
		}

		// no error, update cache & return
		if r.Method == http.MethodGet {
			mu.Lock()
			lastResults[site.GetName()] = result
			mu.Unlock()
		}
		return result, err
	}
}

// withCloudClusterAuth wraps a handler to ensure that a request is  authenticated
// to the remoteSite as specified by the ":site" url parameter (the same as WithClusterAuth),
// and creates a cloud Client to be able to make requests to the cloud API form the remote site.
func (plugin *Plugin) withCloudClusterAuth(next ClusterCloudHandler) httprouter.Handle {
	return plugin.h.WithClusterAuth(func(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, site reversetunnelclient.RemoteSite) (any, error) {
		clt, err := sctx.GetUserClient(r.Context(), site)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ctx, cancel := context.WithTimeout(r.Context(), defaultCloudHTTPTimeout)
		defer cancel()

		client, ok := clt.(*authclient.Client)
		if !ok {
			return nil, trace.BadParameter("unexpected underlying type for auth client")
		}

		cloudClient, err := cloud.NewClientFromConnection(client.GetConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err := next(w, r.WithContext(ctx), sctx, site, cloudClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// if the handler being called as fn returns a protobuf type,
		// encode it using protojson.
		// Otherwise, return the response directly and let our middleware
		// encode it with encoding/json.
		pm, ok := res.(googleproto.Message)
		if ok {
			if err := writeProtoJSONResponse(w, pm); err != nil {
				return nil, trace.Wrap(err)
			}
			return nil, nil
		}

		return res, trace.Wrap(err)
	})
}

// withCloud provides an initialized instance of the cloud client API for public requests.
func (p *Plugin) withCloud(fn cloudPublicHandler) httprouter.Handle {
	return httplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
		client, err := p.getAuthClient()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		ctx, cancel := context.WithTimeout(r.Context(), defaultCloudHTTPTimeout)
		defer cancel()

		cloudClient, err := cloud.NewClientFromConnection(client.GetConnection())
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res, err := fn(w, r.WithContext(ctx), params, cloudClient)
		if err != nil {
			// Hide 429 error.
			if trace.IsLimitExceeded(err) {
				p.Logger.WarnContext(r.Context(), "rate limit exceeded", "error", err)
				return nil, trace.AccessDenied("unable to process your request")
			}
			return nil, trace.Wrap(err)
		}

		return res, nil
	})
}

// writeProtoJSONResponse marshals `pm` into JSON using protojson and writes the result
// to the response writer `w`. If marshaling fails, it doesn't write anything to `w`.
func writeProtoJSONResponse(w http.ResponseWriter, pm googleproto.Message) error {
	result, err := protojson.Marshal(pm)
	if err != nil {
		return trace.Wrap(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result)

	return nil
}
