package web

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/e/lib/scim"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	minSCIMItemIndex     = 1
	defaultSCIMItemCount = 100
	maxSCIMItemCount     = 200

	queryFieldFilter = "filter"
	maxSCIMBodyBytes = 1 * 1024 * 1024
)

func (p *Plugin) registerSCIMHandlers() {
	p.Logger.InfoContext(context.Background(), "Registering SCIM endpoints")

	p.h.GET("/webapi/scim/:integration/:resourceType",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimGetResourceList)))

	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		p.h.Handle(m, "/webapi/scim/:integration/:resourceType",
			p.h.WithUnauthenticatedHighLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))
	}

	p.h.GET("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimGetResource)))

	p.h.POST("/webapi/scim/:integration/:resourceType",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimCreateResource)))

	p.h.PUT("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimUpdateResource)))

	p.h.DELETE("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimDeleteResource)))

	p.h.PATCH("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithUnauthenticatedHighLimiter(
			p.wrapSCIMRequest(p.scimPatchResource)))

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		p.h.Handle(m, "/webapi/scim",
			p.h.WithUnauthenticatedHighLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))

		p.h.Handle(m, "/webapi/scim/:integration",
			p.h.WithUnauthenticatedHighLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))
	}
}

func (p *Plugin) wrapSCIMRequest(fn func(http.ResponseWriter, *http.Request, httprouter.Params) error) httplib.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		p.Logger.Log(r.Context(), logutils.TraceLevel, "Handling SCIM request", "method", r.Method, "url", r.URL, teleport.ComponentKey, "scim")

		var err error
		features := p.h.GetClusterFeatures()
		identity := modules.GetProtoEntitlement(&features, entitlements.OktaSCIM)
		if !identity.Enabled {
			err = trace.AccessDenied("SCIM support requires Teleport Identity")
		} else {
			err = fn(w, r, params)
		}

		if err == nil {
			return nil, nil
		}

		var statusCode int
		switch {
		case trace.IsNotFound(err):
			statusCode = http.StatusNotFound

		case trace.IsAccessDenied(err):
			statusCode = http.StatusUnauthorized

		case trace.IsBadParameter(err):
			statusCode = http.StatusBadRequest

		case trace.IsLimitExceeded(err):
			statusCode = http.StatusRequestEntityTooLarge

		case trace.IsNotImplemented(err):
			statusCode = http.StatusNotImplemented

		default:
			statusCode = http.StatusInternalServerError
		}

		detail := err.Error()

		body, err := scimsdk.FormatErrorResponse(statusCode, detail)
		if err != nil {
			p.Logger.ErrorContext(r.Context(), "failed formatting SCIM error response")
		}
		writeSCIMResponse(w, statusCode, body)

		return nil, nil
	}
}

func (p *Plugin) scimGetResourceList(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
	)

	filter := r.URL.Query().Get(queryFieldFilter)
	if filter != "" {
		// validate the filter syntax is correct and supported. No point in
		// sending the whole request over to auth only for it to be rejected
		// straight away.
		if _, err := scim.ParseFilter(filter); err != nil {
			return trace.BadParameter("unsupported filter syntax")
		}
	}

	page, err := getSCIMPage(r)
	if err != nil {
		return trace.BadParameter("invalid page request")
	}

	log.DebugContext(r.Context(), "Listing resources", "filter", filter, "start", page.StartIndex, "count", page.Count)

	scimClient := p.h.GetProxyClient().SCIMClient()
	resources, err := scimClient.ListSCIMResources(r.Context(), &scimpb.ListSCIMResourcesRequest{
		Target: &scimpb.RequestTarget{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
		},
		Page:   page,
		Filter: filter,
	})

	if err != nil {
		log.ErrorContext(r.Context(), "Failed listing resources", "error", err)
		return trace.Wrap(err)
	}

	body, err := scimsdk.MarshalResourceList(resources)
	if err != nil {
		return trace.Wrap(err)
	}

	writeSCIMResponse(w, http.StatusOK, body)

	return nil
}

func (p *Plugin) scimGetResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID, err := url.QueryUnescape(params.ByName("resourceID"))
	if err != nil {
		return trace.Wrap(err)
	}

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	scimClient := p.h.GetProxyClient().SCIMClient()
	resource, err := scimClient.GetSCIMResource(r.Context(), &scimpb.GetSCIMResourceRequest{
		Target: &scimpb.RequestTarget{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		},
	})
	if err != nil {
		log.ErrorContext(r.Context(), "Failed fetching resource", "error", err)
		return trace.Wrap(err)
	}

	body, err := scimsdk.MarshalResource(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusOK, body)
	return nil
}

func (p *Plugin) scimCreateResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
	)

	if r.ContentLength > maxSCIMBodyBytes {
		return trace.LimitExceeded("content length")
	}

	res, err := scimsdk.UnmarshalResource(&io.LimitedReader{R: r.Body, N: maxSCIMBodyBytes})
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(r.Context(), "Creating new resource")

	scimClient := p.h.GetProxyClient().SCIMClient()
	updated, err := scimClient.CreateSCIMResource(r.Context(), &scimpb.CreateSCIMResourceRequest{
		Target: &scimpb.RequestTarget{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
		},
		Resource: res,
	})
	if err != nil {
		log.ErrorContext(r.Context(), "Failed creating new resource", "error", err)
		return trace.Wrap(err)
	}

	body, err := scimsdk.MarshalResource(updated)
	if err != nil {
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusOK, body)
	return nil
}

func (p *Plugin) scimUpdateResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID, err := url.QueryUnescape(params.ByName("resourceID"))
	if err != nil {
		return trace.Wrap(err)
	}

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	if r.ContentLength > maxSCIMBodyBytes {
		return trace.LimitExceeded("content length")
	}

	res, err := scimsdk.UnmarshalResource(&io.LimitedReader{R: r.Body, N: maxSCIMBodyBytes})
	if err != nil {
		return trace.Wrap(err)
	}

	log.InfoContext(r.Context(), "Updating resource")

	scimClient := p.h.GetProxyClient().SCIMClient()
	updated, err := scimClient.UpdateSCIMResource(r.Context(), &scimpb.UpdateSCIMResourceRequest{
		Target: &scimpb.RequestTarget{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		},
		Resource: res,
	})
	if err != nil {
		log.ErrorContext(r.Context(), "Failed updating resource", "error", err)
		return trace.Wrap(err)
	}

	body, err := scimsdk.MarshalResource(updated)
	if err != nil {
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusOK, body)
	return nil
}

func (p *Plugin) scimDeleteResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID, err := url.QueryUnescape(params.ByName("resourceID"))
	if err != nil {
		return trace.Wrap(err)
	}

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	log.InfoContext(r.Context(), "Deleting resource")

	scimClient := p.h.GetProxyClient().SCIMClient()
	_, err = scimClient.DeleteSCIMResource(r.Context(), &scimpb.DeleteSCIMResourceRequest{
		Target: &scimpb.RequestTarget{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		},
	})

	if err != nil {
		log.ErrorContext(r.Context(), "Failed deleting resource", "error", err)
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusNoContent, nil)

	return nil
}

// scimPatchResource handles a PATCH request on a SCIM resource. We do not
// currently support PATCH requests as our target SCIM clients do not use it
// (e.g. Okta SAML App), so this method merely logs the request for
// troubleshooting purposes and returns NotImplemented.
func (p *Plugin) scimPatchResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID, err := url.QueryUnescape(params.ByName("resourceID"))
	if err != nil {
		return trace.Wrap(err)
	}

	p.Logger.InfoContext(r.Context(), "Unexpected PATCH resource request",
		teleport.ComponentKey, "scim",
		"method", r.Method,
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	return trace.NotImplemented(http.MethodPatch)
}

func (p *Plugin) scimLogRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	p.Logger.InfoContext(r.Context(), "Unexpected SCIM request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.Query(),
	)

	return trace.NotImplemented(http.MethodPatch)
}

func writeSCIMResponse(w http.ResponseWriter, statusCode int, body []byte) {
	if len(body) > 0 {
		w.Header().Set(scimsdk.ContentTypeHeader, scimsdk.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(statusCode)
	w.Write(body)
}

func getSCIMPage(r *http.Request) (*scimpb.Page, error) {
	startIndex, err := getQueryIntOrDefault(r, "startIndex", minSCIMItemIndex)
	if err != nil {
		return nil, trace.Wrap(err, "startIndex")
	}
	startIndex = max(startIndex, minSCIMItemIndex)

	count, err := getQueryIntOrDefault(r, "count", defaultSCIMItemCount)
	if err != nil {
		return nil, trace.Wrap(err, "count")
	}
	// According to spec, all values < 0 must be treated as 0
	count = max(count, 0)

	// We don't want someone asking us for a billion items, so
	// we put an upper bound on the number of records we're prepared to
	// return in one page
	count = min(count, maxSCIMItemCount)

	return &scimpb.Page{
		StartIndex: uint64(startIndex),
		Count:      uint64(count),
	}, nil
}

func getQueryIntOrDefault(r *http.Request, key string, def int) (int, error) {
	text := r.URL.Query().Get(key)
	if text == "" {
		return def, nil
	}

	if n, err := strconv.Atoi(text); err == nil {
		return n, nil
	}

	return 0, trace.BadParameter("not a number: %q", text)
}
