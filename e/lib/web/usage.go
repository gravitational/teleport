package web

import (
	"net/http"

	"github.com/gravitational/trace"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/e/api/cloud"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/web"
)

// getNonBillableUsageSummaryHandle returns usage report for resources that are not tracked in Stripe.
func (p *Plugin) getNonBillableUsageSummaryHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (any, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	usageResp, err := authClt.ResourceUsageClient().GetUsage(r.Context(), &resourceusagepb.GetUsageRequest{})
	if trace.IsAccessDenied(err) {
		// Do not fail the whole request on "access denied", since this endpoint is also called in the "New Request" page,
		// where we always want to display the access request limit for better UX.
		usageResp = &resourceusagepb.GetUsageResponse{}
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.NonBillableUsageSummary{
		AccessRequestUsage: ui.AccessRequestUsage{
			MonthlyLimit: usageResp.GetAccessRequests().GetMonthlyLimit(),
			MonthlyUsed:  usageResp.GetAccessRequests().GetMonthlyUsed(),
		},
	}, nil
}
