package web

import (
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web"
)

func (p *Plugin) getAccessLists(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListClt := clt.AccessListClient()

	// This will grab all access lists but in small chunks to not overload the grpc client.
	// The web UI won't require "paginating" because we don't expect access lists to get
	// in the thousands.
	var accessLists []*accesslist.AccessList
	var nextKey string
	for {
		var page []*accesslist.AccessList
		var err error
		page, nextKey, err = accessListClt.ListAccessLists(r.Context(), 0, nextKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		accessLists = append(accessLists, page...)

		if nextKey == "" {
			break
		}
	}

	return ui.AccessListResponse{
		AccessLists: accessLists,
	}, nil
}

func (p *Plugin) getAccessList(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessListId := params.ByName("accessListId")

	accessList, err := clt.AccessListClient().GetAccessList(r.Context(), accessListId)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AccessListResponse{
		AccessList: accessList,
	}, nil
}

func (p *Plugin) createAccessList(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *web.SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	acessListClt := clt.AccessListClient()

	var req ui.CreateAccessListRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the title to create the resource name.
	req.Title = strings.TrimSpace(req.Title)
	accessListName := strings.ReplaceAll(req.Title, " ", "-")

	auditDuration, err := time.ParseDuration(req.AuditDuration)
	if err != nil {
		return nil, trace.BadParameter("invalid audit duration format: %v", err)
	}
	req.Spec.Audit.Frequency = auditDuration

	accessList, err := accesslist.NewAccessList(header.Metadata{Name: accessListName}, req.Spec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createdAccessList, err := acessListClt.UpsertAccessList(r.Context(), accessList)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.AccessListResponse{
		AccessList: createdAccessList,
	}, nil
}
