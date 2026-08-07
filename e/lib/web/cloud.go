package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
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
// a cloudClient authenticated against a cluster as specified by the ":site" url parameter.
type ClusterCloudHandler func(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error)

func (p *Plugin) registerCloudHandlers() {
	features := p.h.GetClusterFeatures()

	// the billing summary API is available for cloud users and usage-based self-hosted (dashboard) customers
	if features.GetCloud() || (services.IsDashboard(features) && features.IsUsageBased && !features.IsStripeManaged) {
		p.h.GET("/enterprise/cloud/billing-summary", p.withCloudAuth(p.withCloudCache(p.getBillingSummaryInformationHandle)))
		p.h.POST("/enterprise/cloud/billing-summary", p.withCloudAuth(p.withCloudCache(p.getUsageHandle)))
		p.h.GET("/enterprise/cloud/billing/breakdown/mau", p.withCloudAuth(p.getMAUBreakdownHandle))
		p.h.GET("/enterprise/cloud/billing/breakdown/tpr", p.withCloudAuth(p.getTPRBreakdownHandle))
	}

	// the following endpoints are only available to Cloud cloud-hosted customers (not Cloud dashboard customers)
	if features.GetCloud() {
		p.h.GET("/enterprise/cloud/billing", p.withCloudAuth(p.withCloudCache(p.getBillingInformationHandle)))
		p.h.GET("/enterprise/cloud/nonbillable-summary", p.withCloudAuth(p.withCloudCache(p.getNonBillableUsageSummaryHandle)))
	}

	// upgrade window endpoints with cluster param
	p.h.GET("/enterprise/sites/:site/upgradewindowstart", p.withCloudClusterAuth(p.withCloudClusterCache(p.getClusterUpgradeWindowStartHourHandle)))
	p.h.POST("/enterprise/sites/:site/upgradewindowstart", p.withCloudClusterAuth(p.updateClusterUpgradeWindowStartHourHandle))

	// update environment profile
	p.h.GET("/enterprise/cloud/environmentprofile", p.withCloudAuth(p.withCloudCache(p.getEnvironmentProfileHandle)))
	p.h.POST("/enterprise/cloud/environmentprofile", p.withCloudAuth(p.updateEnvironmentProfileHandle))

	// contact endpoints
	p.h.GET("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.withCloudClusterCache(p.getClusterContactHandle)))
	p.h.POST("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.createClusterContactHandle))
	p.h.DELETE("/enterprise/sites/:site/contact", p.withCloudClusterAuth(p.deleteClusterContactHandle))

	// client IP restrictions
	p.h.GET("/enterprise/sites/:site/clientiprestrictions", p.withCloudClusterAuth(p.withCloudClusterCache(p.getClientIPRestrictions)))
	p.h.PUT("/enterprise/sites/:site/clientiprestrictions", p.withCloudClusterAuth(p.putClientIPRestrictions))

	// assets
	p.h.GET("/enterprise/cloud/assets/*path", p.withCloud(p.getCloudAssetHandle))
}

// TODO(michellescripts) safe to remove in v19
// Deprecated by getUsageHandle
func (p *Plugin) getBillingSummaryInformationHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetBillingSummaryInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getUsageHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
	var req cloudapi.GetUsageRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := client.GetUsage(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getMAUBreakdownHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
	window, err := parseDailyBreakdownWindow(r.URL.Query())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := &cloudapi.GetMAUDailyBreakdownRequest{
		Tenants: r.URL.Query()["tenants"],
		Window:  window,
	}

	res, err := client.GetMAUDailyBreakdown(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getTPRBreakdownHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, client cloud.Client) (any, error) {
	window, err := parseDailyBreakdownWindow(r.URL.Query())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := &cloudapi.GetTPRDailyBreakdownRequest{
		Tenants: r.URL.Query()["tenants"],
		Window:  window,
	}

	res, err := client.GetTPRDailyBreakdown(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

// parseDailyBreakdownWindow builds a DailyBreakdownWindow from query params.
// Accepts either window-cycle=<int64> or window-range-start=<int64>&window-range-end=<int64>.
// When using a range window, start must be before end.
func parseDailyBreakdownWindow(query url.Values) (*cloudapi.DailyBreakdownWindow, error) {
	if cycleStr := query.Get("window-cycle"); cycleStr != "" {
		cycle, err := strconv.ParseInt(cycleStr, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("cannot convert cycle start '%s' into unix timestamp", cycleStr)
		}

		return &cloudapi.DailyBreakdownWindow{
			Window: &cloudapi.DailyBreakdownWindow_Cycle{Cycle: cycle},
		}, nil
	}

	startStr := query.Get("window-range-start")
	endStr := query.Get("window-range-end")
	if startStr != "" || endStr != "" {
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("cannot convert start date %q into unix timestamp", startStr)
		}
		end, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil {
			return nil, trace.BadParameter("cannot convert end data %q into unix timestamp", endStr)
		}

		if start > end {
			return nil, trace.BadParameter("start date %q must come before end date %q", startStr, endStr)
		}

		return &cloudapi.DailyBreakdownWindow{
			Window: &cloudapi.DailyBreakdownWindow_Range{
				Range: &cloudapi.DateRange{Start: start, End: end},
			},
		}, nil
	}

	return nil, trace.BadParameter("window must have either range or cycle")
}

func (p *Plugin) getBillingInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	res, err := client.GetBillingInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getClusterUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
	res, err := cloudClient.GetAccountUpgradeWindowStartHour(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updateClusterUpgradeWindowStartHourHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
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

func (p *Plugin) getEnvironmentProfileHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, cloudClient cloud.Client) (any, error) {
	res, err := cloudClient.GetEnvironmentProfile(r.Context(), &cloudapi.GetEnvironmentProfileRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updateEnvironmentProfileHandle(w http.ResponseWriter, r *http.Request, _ *web.SessionContext, cloudClient cloud.Client) (any, error) {
	var req cloudapi.UpdateEnvironmentProfileRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := cloudClient.UpdateEnvironmentProfile(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return updated, nil
}

func (p *Plugin) getClientIPRestrictions(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
	res, err := cloudClient.GetClientIPRestrictions(r.Context(), &cloudapi.GetClientIPRestrictionsRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res.ClientIpRestrictions, nil
}

func (p *Plugin) putClientIPRestrictions(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
	var req cloudapi.PutClientIPRestrictionsRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := cloudClient.PutClientIPRestrictions(r.Context(), &req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return res.ClientIpRestrictions, nil
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

func (p *Plugin) getClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
	contacts, err := cloudClient.GetContacts(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}
	return contacts, nil
}

func (p *Plugin) createClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
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

func (p *Plugin) deleteClusterContactHandle(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
	var req cloudapi.RemoveContactRequest
	if err := p.readProtoJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if _, err := cloudClient.RemoveContact(r.Context(), &req); err != nil {
		return nil, trail.FromGRPC(err)
	}
	return web.OK(), nil
}

func (p *Plugin) getCloudAssetHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, client cloud.Client) (any, error) {
	filePath := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/enterprise/cloud/assets"), "/")

	stream, err := client.GetFile(r.Context(), &cloudapi.GetFileRequest{
		Filepath: filePath,
	})
	if err != nil {
		p.Logger.WarnContext(r.Context(), "Failed to fetch cloud panel asset", "error", err, "path", filePath)
		return nil, trail.FromGRPC(err)
	}

	headersWritten := false
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			p.Logger.WarnContext(r.Context(), "Failed to fetch cloud panel asset", "error", err, "path", filePath)
			return nil, trail.FromGRPC(err)
		}

		// Set response headers from the first chunk, before writing any body bytes.
		if !headersWritten {
			ext := filepath.Ext(filePath)
			w.Header().Set("Content-Type", mime.TypeByExtension(ext))
			if chunk.ContentEncoding != "" {
				w.Header().Set("Content-Encoding", chunk.ContentEncoding)
			}
			headersWritten = true
		}

		if len(chunk.Data) > 0 {
			if _, err := w.Write(chunk.Data); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}

	// Return nil to prevent the middleware from overwriting the response with application/json.
	return nil, nil
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

	return func(w http.ResponseWriter, r *http.Request, sctx *web.SessionContext, cluster reversetunnelclient.Cluster, cloudClient cloud.Client) (any, error) {
		result, err := fn(w, r, sctx, cluster, cloudClient)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return nil, err
			}
			if r.Method == http.MethodGet {
				mu.Lock()
				defer mu.Unlock()
				if v := lastResults[cluster.GetName()]; v != nil {
					p.Logger.WarnContext(r.Context(), "Unable to get Cloud response, returning last cached value", "error", err, "url", r.URL.String())
					return v, nil
				}
			}
			return result, err
		}

		// no error, update cache & return
		if r.Method == http.MethodGet {
			mu.Lock()
			lastResults[cluster.GetName()] = result
			mu.Unlock()
		}
		return result, err
	}
}

// withCloudClusterAuth wraps a handler to ensure that a request is authenticated
// to the cluster as specified by the ":site" url parameter (the same as WithClusterAuth),
// and creates a cloud Client to be able to make requests to the cloud API from the cluster.
func (plugin *Plugin) withCloudClusterAuth(next ClusterCloudHandler) httprouter.Handle {
	return plugin.h.WithClusterAuth(func(w http.ResponseWriter, r *http.Request, p httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
		clt, err := sctx.GetUserClient(r.Context(), cluster)
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

		res, err := next(w, r.WithContext(ctx), sctx, cluster, cloudClient)
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
