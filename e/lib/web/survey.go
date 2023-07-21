package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"

	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

type setSurveyResultsReq struct {
	CompanyName   string   `json:"companyName"`
	EmployeeCount string   `json:"employeeCount"`
	Resources     []string `json:"resources"`
	Role          string   `json:"role"`
	Team          string   `json:"team"`
}

func (p *Plugin) surveyResultsHandler(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req setSurveyResultsReq
	if err := httplib.ReadJSON(r, &req); err != nil {
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
