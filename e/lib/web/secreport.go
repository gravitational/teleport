package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/proto"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	usageeventsv1 "github.com/gravitational/teleport/api/gen/proto/go/usageevents/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) listSecurityReports(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	resp, err := client.GetSecurityReports(r.Context())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convSecurityReports(resp), nil
}

func (p *Plugin) getSecurityReport(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	resp, err := client.GetSecurityReport(r.Context(), params.ByName("name"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Spec, trace.Wrap(err)
}

func (p *Plugin) getSecurityReportResult(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	days, err := parseDaysFromParam(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reportName := params.ByName("name")

	event := &proto.SubmitUsageEventRequest{
		Event: &usageeventsv1.UsageEventOneOf{
			Event: &usageeventsv1.UsageEventOneOf_SecurityReportGetResult{
				SecurityReportGetResult: &usageeventsv1.SecurityReportGetResultEvent{
					Name: reportName,
					Days: int32(days),
				},
			},
		},
	}
	if err := clt.SubmitUsageEvent(r.Context(), event); err != nil {
		p.Logger.WarnContext(r.Context(), "Failed to emit usage event", "error", err)
	}

	client := clt.SecReportsClient()
	resp, err := client.GetSecurityReportResult(r.Context(), reportName, days)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, trace.Wrap(err)
}

func parseDaysFromParam(params httprouter.Params) (int, error) {
	const defaultDays = 7
	if daysParam := params.ByName("days"); daysParam != "" {
		val, err := strconv.Atoi(daysParam)
		if err != nil {
			return 0, trace.Wrap(err)
		}
		return val, nil
	}
	return defaultDays, nil
}

func convSecurityReports(reports []*secreports.Report) []secreports.ReportSpec {
	out := make([]secreports.ReportSpec, 0, len(reports))
	for _, v := range reports {
		out = append(out, v.Spec)
	}
	return out
}

func (p *Plugin) getSchema(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	resp, err := client.GetSchema(r.Context())
	return resp, trace.Wrap(err)
}

func (p *Plugin) runSecurityReport(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Body.Close()
	var req ui.RunReportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	if err := client.RunSecurityReport(r.Context(), params.ByName("name"), req.Days); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil

}

func (p *Plugin) getAuditQuery(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	resp, err := client.GetSecurityAuditQuery(r.Context(), params.ByName("name"))
	if err != nil {
		return nil, trace.Wrap(err)

	}
	return resp.Spec, trace.Wrap(err)
}

func (p *Plugin) listAuditQueries(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	resp, err := client.GetSecurityAuditQueries(r.Context())
	return ui.ConvAuditQueries(resp), trace.Wrap(err)
}

func (p *Plugin) runAuditQuery(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Always drain the body, regardless of the status code.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Body.Close()
	var req ui.RunAuditQueryRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	resp, err := client.RunAuditQuery(r.Context(), req.Query, req.Days)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

func (p *Plugin) upsertAuditQuery(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Body.Close()
	var spec secreports.AuditQuerySpec
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, trace.Wrap(err)
	}
	auditQuery, err := secreports.NewAuditQuery(header.Metadata{Name: spec.Name}, spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	if err := client.UpsertSecurityAuditQuery(r.Context(), auditQuery); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil
}

func (p *Plugin) deleteAuditQuery(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client := clt.SecReportsClient()
	if err := client.DeleteSecurityAuditQuery(r.Context(), params.ByName("name")); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil
}

func (p *Plugin) upsertSecurityReport(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Always drain the body, regardless of the status code.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Body.Close()
	var req secreports.ReportSpec
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	item, err := secreports.NewReport(header.Metadata{Name: req.Name}, secreports.ReportSpec{
		Name:         req.Name,
		Title:        req.Title,
		Description:  req.Description,
		AuditQueries: req.AuditQueries,
		Version:      req.Version,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	if err := client.UpsertSecurityReport(r.Context(), item); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil
}

func (p *Plugin) deleteSecurityReport(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	if err := client.DeleteSecurityReport(r.Context(), params.ByName("name")); err != nil {
		return nil, trace.Wrap(err)
	}
	return nil, nil
}

func (p *Plugin) getSecurityReportState(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	var days = 7
	if daysParam := params.ByName("days"); daysParam != "" {
		val, err := strconv.Atoi(daysParam)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		days = val
	}
	resp, err := client.GetSecurityReportExecutionState(r.Context(), params.ByName("name"), int32(days))
	if err != nil {
		return nil, trace.Wrap(err)

	}
	return resp.Spec, trace.Wrap(err)
}

func (p *Plugin) getQueryResult(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	clt, err := ctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var req pb.GetAuditQueryResultRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer r.Body.Close()
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	client := clt.SecReportsClient()
	resp, err := client.GetSecurityAuditQueryResult(r.Context(), req.GetResultId(), req.GetNextToken(), req.GetMaxResults())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, trace.Wrap(err)
}
