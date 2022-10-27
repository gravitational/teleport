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

func (p *Plugin) listBillingCyclesHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	response, err := client.ListBillingCycles(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response.Cycles, nil
}

func (p *Plugin) listInvoicesHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	response, err := client.ListInvoices(r.Context(), &cloudapi.EmptyRequest{})
	if err != nil {
		return nil, trail.FromGRPC(err)
	}

	return response.Invoices, nil
}

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

func (p *Plugin) updateAccountHandle(w http.ResponseWriter, r *http.Request, ctx *web.SessionContext, client cloud.Client) (interface{}, error) {
	var req *cloudapi.UpdateAccountRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := client.UpdateAccount(r.Context(), req)
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
