package web

import (
	"net/http"

	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"

	resourceusagepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/resourceusage/v1"
	"github.com/gravitational/teleport/e/api/cloud"
	cloudapi "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) removeCardHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.RemoveCardRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := client.RemoveCard(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

func (p *Plugin) addCardHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.AddCardRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := client.AddCard(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

func (p *Plugin) updateCardHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.UpdateCardRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := client.UpdateCard(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

func (p *Plugin) updateStripeAddressHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.StripeBillingAddressRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := client.UpdateStripeAddress(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

func (p *Plugin) getBillingInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetBillingInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) cancelSubscriptionHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	response, err := client.CancelSubscription(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response, nil
}

func (p *Plugin) getBillingSummaryInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetBillingSummaryInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getPaymentsInvoicesInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetPaymentsInvoicesInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) getInvoiceSettingsInformationHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.GetInvoiceSettingsInformation(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) createSetupIntentHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	res, err := client.CreateSetupIntent(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updatePurchaseOrderHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.UpdatePurchaseOrderPrefixRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := client.UpdatePurchaseOrderPrefix(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

func (p *Plugin) updateEmailHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.UpdateEmailRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := client.UpdateEmail(r.Context(), req)
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return res, nil
}

// getNonBillableUsageSummaryHandle returns usage report for resources that are not tracked in Stripe.
func (p *Plugin) getNonBillableUsageSummaryHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	authClt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Important: GetDevicesUsage only returns non-zeroed usage values for
	// usage-based accounts.
	// See [devicepb.DevicesUsage.AccountUsageType] (or handle the zeroes
	// accordingly!)
	usageResp, err := authClt.ResourceUsageClient().GetUsage(r.Context(), &resourceusagepb.GetUsageRequest{})
	if trace.IsAccessDenied(err) {
		// Do not fail the whole request on "access denied", since this endpoint is also called in the "New Request" page,
		// where we always want to display the access request limit for better UX.
		usageResp = &resourceusagepb.GetUsageResponse{}
	} else if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ui.NonBillableUsageSummary{
		TrustedDeviceUsage: ui.TrustedDeviceUsage{
			DevicesUsageLimit: usageResp.GetDevicesUsage().GetDevicesUsageLimit(),
			DevicesInUse:      usageResp.GetDevicesUsage().GetDevicesInUse(),
		},
		AccessRequestUsage: ui.AccessRequestUsage{
			MonthlyLimit: usageResp.GetAccessRequests().GetMonthlyLimit(),
			MonthlyUsed:  usageResp.GetAccessRequests().GetMonthlyUsed(),
		},
	}, nil
}
