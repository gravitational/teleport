package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/gravitational/teleport"
	scimclient "github.com/gravitational/teleport/api/client/scim"
	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	scimpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	scimfilter "github.com/gravitational/teleport/e/lib/scim/service/filter"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/modules"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	minSCIMItemIndex     = 1
	defaultSCIMItemCount = 100
	maxSCIMItemCount     = 200

	queryFieldFilter = "filter"

	defaultMaxSCIMBodyBytes = 3 * 1024 * 1024

	// scimMaxBodyBytesEnvVar is the environment variable used to override
	// defaultMaxSCIMBodyBytes.
	scimMaxBodyBytesEnvVar = "TELEPORT_UNSTABLE_SCIM_MAX_BODY_BYTES"
)

// getMaxSCIMBodyBytes returns the maximum accepted size, in bytes, of a SCIM
// request body, read from scimMaxBodyBytesEnvVar (falling back to
// defaultMaxSCIMBodyBytes if unset or invalid).
func getMaxSCIMBodyBytes() int64 {
	v := os.Getenv(scimMaxBodyBytesEnvVar)
	if v == "" {
		return defaultMaxSCIMBodyBytes
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return defaultMaxSCIMBodyBytes
	}
	return n
}

func (p *Plugin) registerSCIMHandlers() {
	p.Logger.InfoContext(context.Background(), "Registering SCIM endpoints")

	p.h.GET("/webapi/scim/:integration/:resourceType",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimGetResourceList)))

	for _, m := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
		p.h.Handle(m, "/webapi/scim/:integration/:resourceType",
			p.h.WithAccessDeniedLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))
	}

	p.h.GET("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimGetResource)))

	p.h.POST("/webapi/scim/:integration/:resourceType",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimCreateResource)))

	p.h.PUT("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimUpdateResource)))

	p.h.DELETE("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimDeleteResource)))

	p.h.PATCH("/webapi/scim/:integration/:resourceType/:resourceID",
		p.h.WithAccessDeniedLimiter(
			p.wrapSCIMRequest(p.scimPatchResource)))

	for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		p.h.Handle(m, "/webapi/scim",
			p.h.WithAccessDeniedLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))

		p.h.Handle(m, "/webapi/scim/:integration",
			p.h.WithAccessDeniedLimiter(
				p.wrapSCIMRequest(p.scimLogRequest)))
	}

	p.h.POST("/webapi/plugin/:plugin_name/token",
		p.h.WithUnauthenticatedHighLimiter(p.getToken))
}

// checkSCIMRequestAllowed returns an error if the request should be rejected
// before reaching its handler, e.g. because SCIM isn't licensed for the
// cluster or the request body is larger than we're willing to accept.
func (p *Plugin) checkSCIMRequestAllowed(r *http.Request) error {
	identity := modules.GetProtoEntitlement(new(p.h.GetClusterFeatures()), entitlements.OktaSCIM)
	if !identity.Enabled {
		return trace.AccessDenied("SCIM support requires Teleport Identity Governance")
	}
	return checkSCIMBodyLength(r)
}

func (p *Plugin) wrapSCIMRequest(fn func(http.ResponseWriter, *http.Request, httprouter.Params) error) httplib.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (any, error) {
		p.Logger.Log(r.Context(), logutils.TraceLevel, "Handling SCIM request", "method", r.Method, "url", r.URL, teleport.ComponentKey, "scim")

		err := p.checkSCIMRequestAllowed(r)
		if err == nil {
			err = fn(w, r, params)
		}

		if err == nil {
			return nil, nil
		}

		var statusCode int
		switch {
		case trace.IsNotFound(err):
			statusCode = http.StatusNotFound

		case trace.IsAlreadyExists(err):
			statusCode = http.StatusConflict

		case trace.IsAccessDenied(err):
			statusCode = http.StatusUnauthorized

		case trace.IsBadParameter(err):
			statusCode = http.StatusBadRequest

		case trace.IsLimitExceeded(err):
			var rlErr *scimclient.RateLimitError
			if errors.As(err, &rlErr) {
				// This is explicit rate limiting error returned by the client.
				// The IsLimitExceeded with RateLimitError error should be mapped to StatusTooManyRequests.
				statusCode = http.StatusTooManyRequests
				if rlErr.RetryAfterSeconds > 0 {
					w.Header().Set("Retry-After", strconv.FormatInt(rlErr.RetryAfterSeconds, 10))
				}
			} else {
				// Standalone IsLimitExceeded means that the request body is too large, which is a bad request.
				// This is different from the rate limit error above which is returned by the client when it detects that it's being rate limited by the server
				// We want to return BadRequest in this case to be consistent with the behavior when the request body exceeds the max size limit defined in the handler
				statusCode = http.StatusBadRequest
			}

		case trace.IsNotImplemented(err):
			statusCode = http.StatusNotImplemented

		case trace.IsCompareFailed(err):
			statusCode = http.StatusPreconditionFailed

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

func (p *Plugin) scimGetResourceList(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
	)

	// ServiceProviderConfig is a singleton resource as defined in RFC 7643 §8.5:
	// https://datatracker.ietf.org/doc/html/rfc7643#section-8.5
	//
	// Unlike other endpoints, it must return a single SCIM resource—not a list.
	// However, our internal gRPC SCIM provider interface uses ListResources for all handlers.
	// To bridge this mismatch, we explicitly redirect the request to scimGetResource,
	// which handles returning the correct single-resource response.
	if resourceType == "ServiceProviderConfig" {
		return p.scimGetResource(w, r, params)
	}

	filter := r.URL.Query().Get(queryFieldFilter)
	if filter != "" {
		// validate the filter syntax is correct and supported. No point in
		// sending the whole request over to auth only for it to be rejected
		// straight away.
		if _, err := scimfilter.ParseFilter(filter); err != nil {
			return trace.BadParameter("unsupported filter syntax")
		}
	}

	page, err := getSCIMPage(r)
	if err != nil {
		return trace.BadParameter("invalid page request")
	}

	// We don't emit any audit events before this point as it's an easy vector
	// for a DoS attack.
	event := scimNewListAuditEvent(integration, resourceType, r)
	defer func() {
		// Don't emit an audit event for access denied as it's an way to spam the
		// audit log as a DoS attack.
		if trace.IsAccessDenied(requestError) {
			return
		}
		if requestError != nil {
			event.Metadata.Code = events.SCIMListResourcesFailureCode
			event.Status.Success = false
			event.Status.Error = requestError.Error()
		}
		if emitErr := p.EmitAuditEvent(r.Context(), event); emitErr != nil {
			log.ErrorContext(r.Context(), "failed emitting audit event",
				"error", emitErr)
		}
	}()
	event.Filter = filter
	event.StartIndex = int32(page.rawStartIndex)
	event.Count = int32(page.rawCount)

	log.DebugContext(r.Context(), "Listing resources",
		"filter", filter,
		"start", page.validatedPage.GetStartIndex(),
		"count", page.validatedPage.GetCount())

	scimClient := p.h.GetProxyClient().SCIMClient()
	resources, err := scimClient.ListSCIMResources(r.Context(), scimpb.ListSCIMResourcesRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
		}.Build(),
		Page:   page.validatedPage,
		Filter: filter,
	}.Build())

	if err != nil {
		log.ErrorContext(r.Context(), "Failed listing resources", "error", err)
		return trace.Wrap(err)
	}

	event.ResourceCount = uint32(len(resources.GetResources()))

	body, err := scimsdk.MarshalResourceList(resources)
	if err != nil {
		return trace.Wrap(err)
	}

	writeSCIMResponse(w, http.StatusOK, body)

	return nil
}

func (p *Plugin) scimGetResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	ctx := r.Context()
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID := params.ByName("resourceID")

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)
	log.InfoContext(r.Context(), "SCIM Get resource")

	auditEvent := scimNewResourceAuditEvent(integration, resourceType, r, events.SCIMGetEvent, events.SCIMGetResourceSuccessCode)
	defer func() {
		err := scimEmitResourceEvent(ctx, p, auditEvent, requestError, events.SCIMGetResourceFailureCode)
		if err != nil {
			log.ErrorContext(ctx, "Failed emitting audit event", "error", err)
		}
	}()
	auditEvent.TeleportID = resourceID

	scimClient := p.h.GetProxyClient().SCIMClient()
	resource, err := scimClient.GetSCIMResource(ctx, scimpb.GetSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		}.Build(),
	}.Build())
	if err != nil {
		log.ErrorContext(r.Context(), "Failed fetching resource", "error", err)
		return trace.Wrap(err)
	}
	auditEvent.ExternalID = resource.GetExternalId()
	auditEvent.Display = extractDisplayName(resource)

	body, err := scimsdk.MarshalResource(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusOK, body,
		withETag(resource.GetMeta().GetVersion()))
	return nil
}

func extractDisplayName(r *scimpb.Resource) string {
	if !r.HasAttributes() {
		return ""
	}

	displayName, ok := r.GetAttributes().Fields["displayName"]
	if !ok {
		return ""
	}

	return displayName.GetStringValue()
}

// checkSCIMBodyLength returns an error if the request's declared content
// length exceeds getMaxSCIMBodyBytes.
func checkSCIMBodyLength(r *http.Request) error {
	if r.ContentLength <= getMaxSCIMBodyBytes() {
		return nil
	}
	return trace.LimitExceeded("detected an unusually large SCIM request payload (%d bytes, limit %d bytes). "+
		"If you are self-hosting Teleport, you can raise this limit by setting the %s environment variable; "+
		"otherwise, please contact Teleport support and report this error along with the scale of your SCIM environment (e.g. number of users/groups)",
		r.ContentLength, getMaxSCIMBodyBytes(), scimMaxBodyBytesEnvVar)
}

func (p *Plugin) scimCreateResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	ctx := r.Context()
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
	)

	bodyAttribs, err := scimsdk.UnmarshalAttributeSet(&io.LimitedReader{R: r.Body, N: getMaxSCIMBodyBytes()})
	if err != nil {
		return trace.Wrap(err)
	}
	res, err := scimsdk.DecodeResource(bodyAttribs)
	if err != nil {
		return trace.Wrap(err)
	}

	auditEvent := scimNewResourceAuditEvent(integration, resourceType, r, events.SCIMCreateEvent, events.SCIMResourceCreateSuccessCode)
	auditEvent.ExternalID = res.GetExternalId()
	auditEvent.Request.Body, err = apievents.EncodeMap(bodyAttribs)
	if err != nil {
		return trace.Wrap(err, "malformed body JSON")
	}

	// We don't emit any audit events before this point as spamming the auditlog
	// it's an easy vector for a DoS attack.
	defer func() {
		err := scimEmitResourceEvent(ctx, p, auditEvent, requestError, events.SCIMResourceCreateFailureCode)
		if err != nil {
			log.ErrorContext(ctx, "Failed emitting audit event", "error", err)
		}
	}()

	log.DebugContext(ctx, "Creating new resource")

	scimClient := p.h.GetProxyClient().SCIMClient()
	created, err := scimClient.CreateSCIMResource(ctx, scimpb.CreateSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
		}.Build(),
		Resource: res,
	}.Build())
	if err != nil {
		log.ErrorContext(r.Context(), "Failed creating new resource", "error", err)
		return trace.Wrap(err)
	}
	auditEvent.ExternalID = created.GetExternalId()
	auditEvent.TeleportID = created.GetId()
	auditEvent.Display = extractDisplayName(created)

	// Return 201 Created status code
	//
	// According to SCIM RFC https://datatracker.ietf.org/doc/html/rfc7644#section-3.3
	//
	// When the service provider successfully creates the new resource, an
	// HTTP response SHALL be returned with HTTP status code 201 (Created).
	//
	// This is consistent with Okta behavior
	// https://developer.okta.com/docs/api/openapi/okta-scim/guides/scim-20/#create-the-user
	// when 201 is returned when a new user is created.
	err = writeSCIMResourceUpdateResponse(w, http.StatusCreated, created, auditEvent)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *Plugin) scimUpdateResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	ctx := r.Context()
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID := params.ByName("resourceID")

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	bodyAttribs, err := scimsdk.UnmarshalAttributeSet(&io.LimitedReader{R: r.Body, N: getMaxSCIMBodyBytes()})
	if err != nil {
		return trace.Wrap(err)
	}

	res, err := scimsdk.DecodeResource(bodyAttribs)
	if err != nil {
		return trace.Wrap(err)
	}

	auditEvent := scimNewResourceAuditEvent(integration, resourceType, r, events.SCIMUpdateEvent, events.SCIMResourceUpdateSuccessCode)
	auditEvent.TeleportID = resourceID
	auditEvent.Request.Body, err = apievents.EncodeMap(bodyAttribs)
	if err != nil {
		return trace.Wrap(err, "malformed body JSON")
	}

	// We don't emit any audit events before this point as spamming the auditlog
	// it's an easy vector for a DoS attack.
	defer func() {
		err := scimEmitResourceEvent(ctx, p, auditEvent, requestError, events.SCIMResourceUpdateFailureCode)
		if err != nil {
			log.ErrorContext(ctx, "Failed emitting audit event", "error", err)
		}
	}()

	log.InfoContext(r.Context(), "Updating resource")
	scimClient := p.h.GetProxyClient().SCIMClient()
	updated, err := scimClient.UpdateSCIMResource(r.Context(), scimpb.UpdateSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		}.Build(),
		Resource: res,
	}.Build())
	if err != nil {
		log.ErrorContext(r.Context(), "Failed updating resource", "error", err)
		return trace.Wrap(err)
	}
	auditEvent.ExternalID = updated.GetExternalId()
	auditEvent.Display = extractDisplayName(updated)

	err = writeSCIMResourceUpdateResponse(w, http.StatusOK, updated, auditEvent)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *Plugin) scimDeleteResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	ctx := r.Context()
	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID := params.ByName("resourceID")

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	auditEvent := scimNewResourceAuditEvent(integration, resourceType, r, events.SCIMDeleteEvent, events.SCIMResourceDeleteSuccessCode)
	defer func() {
		err := scimEmitResourceEvent(ctx, p, auditEvent, requestError, events.SCIMResourceDeleteFailureCode)
		if err != nil {
			log.ErrorContext(ctx, "Failed emitting audit event", "error", err)
		}
	}()
	auditEvent.TeleportID = resourceID

	log.InfoContext(r.Context(), "Deleting resource")

	scimClient := p.h.GetProxyClient().SCIMClient()
	_, err := scimClient.DeleteSCIMResource(r.Context(), scimpb.DeleteSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		}.Build(),
	}.Build())

	if err != nil {
		log.ErrorContext(r.Context(), "Failed deleting resource", "error", err)
		return trace.Wrap(err)
	}
	writeSCIMResponse(w, http.StatusNoContent, nil)

	return nil
}

// scimPatchResource handles SCIM PATCH requests to partially update a resource.
// See RFC 7644 Section 3.5.2 for details
func (p *Plugin) scimPatchResource(w http.ResponseWriter, r *http.Request, params httprouter.Params) (requestError error) {
	ctx := r.Context()

	integration := params.ByName("integration")
	resourceType := params.ByName("resourceType")
	resourceID := params.ByName("resourceID")

	log := p.Logger.With(
		teleport.ComponentKey, "scim",
		"method", r.Method,
		"integration", integration,
		"resource_type", resourceType,
		"resource_id", resourceID,
	)

	bodyAttribs, err := scimsdk.UnmarshalAttributeSet(&io.LimitedReader{R: r.Body, N: getMaxSCIMBodyBytes()})
	if err != nil {
		return trace.Wrap(err)
	}

	auditEvent := scimNewResourceAuditEvent(integration, resourceType, r, events.SCIMPatchEvent, events.SCIMResourcePatchSuccessCode)
	auditEvent.TeleportID = resourceID
	auditEvent.Request.Body, err = apievents.EncodeMap(bodyAttribs)
	if err != nil {
		return trace.Wrap(err, "malformed body JSON")
	}

	// Set up for automatically emitting an AuditLog event when we exit the handler.
	// We don't emit any audit events on a failure before this point as spamming
	// the auditlog is an easy vector for a DoS attack.
	defer func() {
		err := scimEmitResourceEvent(ctx, p, auditEvent, requestError, events.SCIMResourcePatchFailureCode)
		if err != nil {
			log.ErrorContext(ctx, "Failed emitting audit event", "error", err)
		}
	}()

	log.InfoContext(ctx, "Handling PATCH resource request")

	payload, err := structpb.NewStruct(bodyAttribs)
	if err != nil {
		return trace.Wrap(err)
	}

	authClient, err := p.getAuthClient()
	if err != nil {
		return trace.Wrap(err)
	}
	scimClient := scimclient.NewClientFromConn(authClient.GetConnection())

	resp, err := scimClient.PatchSCIMResourceV2(r.Context(), scimpb.PatchSCIMResourceRequest_builder{
		Target: scimpb.RequestTarget_builder{
			Authorization: r.Header.Get("Authorization"),
			PluginId:      integration,
			ResourceType:  resourceType,
			ResourceId:    resourceID,
		}.Build(),
		Payload: payload,
	}.Build())
	if err != nil {
		return trace.Wrap(err)
	}

	code := http.StatusOK
	// A fast-path patch applied without materializing a representation of
	// the resource - respond 204 No Content, per RFC 7644 Section 3.5.2.
	// The SCIM fast patch path leverage 204 No Content to avoid the overhead of handling
	// large set of members in groups.
	if resp.NoContent {
		code = http.StatusNoContent
	}

	auditEvent.ExternalID = resp.Resource.GetExternalId()
	auditEvent.Display = extractDisplayName(resp.Resource)

	err = writeSCIMResourceUpdateResponse(w, code, resp.Resource, auditEvent)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *Plugin) scimLogRequest(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
	p.Logger.WarnContext(r.Context(), "Unexpected SCIM request",
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.Query(),
	)

	return trace.NotImplemented(http.MethodPatch)
}

func (p *Plugin) getToken(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
	authClient, err := p.getAuthClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pluginClient := pluginspb.NewPluginServiceClient(authClient.GetConnection())

	resp, err := pluginClient.CreatePluginOauthToken(r.Context(), pluginspb.CreatePluginOauthTokenRequest_builder{
		ClientId:     r.FormValue("client_id"),
		ClientSecret: r.FormValue("client_secret"),
		GrantType:    r.FormValue("grant_type"),
		PluginName:   params.ByName("plugin_name"),
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	uiResp := &ui.OauthTokenResponse{
		AccessToken: resp.GetAccessToken(),
		TokenType:   resp.GetTokenType(),
		ExpiresIn:   resp.GetExpiresIn(),
	}
	return uiResp, nil
}

type scimResponseOptions struct {
	etag string
}

type scimResponseOption func(*scimResponseOptions)

func withETag(etag string) scimResponseOption {
	return func(opts *scimResponseOptions) {
		opts.etag = etag
	}
}

// writeSCIMResourceUpdateResponse records the updated resource in the supplied
// audit event before writing the SCIM respinse as usual
func writeSCIMResourceUpdateResponse(w http.ResponseWriter, statusCode int, resource *scimpb.Resource, auditEvent *apievents.SCIMResourceEvent) error {
	// A 204 No Content response has no body, per RFC 7644 Section 3.5.2 -
	// it's only returned when the resource wasn't materialized (the SCIM
	// fast patch path builds `resource` as a placeholder, without fetching
	// group membership, purely to carry metadata like the ETag). Recording
	// it as the audit event's response body would fabricate state - e.g.
	// falsely reporting an empty membership list for a group that still
	// has many members - so leave both the audit event and the actual HTTP
	// response bodyless instead of flattening the placeholder.
	if statusCode == http.StatusNoContent {
		auditEvent.Response = &apievents.SCIMResponse{
			StatusCode: uint32(statusCode),
		}
		writeSCIMResponse(w, statusCode, nil, withETag(resource.GetMeta().GetVersion()))
		return nil
	}

	resourceAttributes, err := scimsdk.FlattenResource(resource)
	if err != nil {
		return trace.Wrap(err)
	}

	eventBody, err := apievents.EncodeMap(resourceAttributes)
	if err != nil {
		return trace.Wrap(err)
	}

	responseBody, err := json.Marshal(&resourceAttributes)
	if err != nil {
		return trace.Wrap(err, "marshaling SCIM resource")
	}

	// TODO(tcsc): Implement truncation for large respionse bodies

	auditEvent.Response = &apievents.SCIMResponse{
		StatusCode: uint32(statusCode),
		Body:       eventBody,
	}

	writeSCIMResponse(w, statusCode, responseBody, withETag(resource.GetMeta().GetVersion()))
	return nil
}

func writeSCIMResponse(w http.ResponseWriter, statusCode int, body []byte, options ...scimResponseOption) {
	opts := scimResponseOptions{}
	for _, applyOption := range options {
		applyOption(&opts)
	}

	// for versioned resources, the server must supply an ETag header which is
	// identical to the SCIM resource version.
	// See https://datatracker.ietf.org/doc/html/rfc7644#section-3.14
	if opts.etag != "" {
		w.Header().Set("ETag", opts.etag)
	}
	if len(body) > 0 {
		w.Header().Set(scimsdk.ContentTypeHeader, scimsdk.ContentType)
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}
	w.WriteHeader(statusCode)
	w.Write(body)
}

type scimPage struct {
	rawStartIndex int
	rawCount      int
	validatedPage *scimpb.Page
}

func getSCIMPage(r *http.Request) (scimPage, error) {
	rawStartIndex, err := getQueryIntOrDefault(r, "startIndex", -1)
	if err != nil {
		return scimPage{}, trace.Wrap(err, "malformed startIndex value")
	}
	startIndex := max(rawStartIndex, minSCIMItemIndex)

	rawCount, err := getQueryIntOrDefault(r, "count", -1)
	if err != nil {
		return scimPage{}, trace.Wrap(err, "malformed count value")
	}

	// if the count was unspecified, use our default value
	count := rawCount
	if count == -1 {
		count = defaultSCIMItemCount
	}

	// According to spec, all values < 0 must be treated as 0
	count = max(0, count)

	// We don't want someone asking us for a billion items, so
	// we put an upper bound on the number of records we're prepared to
	// return in one page
	count = min(count, maxSCIMItemCount)

	return scimPage{
		rawStartIndex: rawStartIndex,
		rawCount:      rawCount,
		validatedPage: scimpb.Page_builder{
			StartIndex: uint64(startIndex),
			Count:      uint64(count),
		}.Build(),
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

func scimNewEventCommonData(pluginName, resourceType string, req *http.Request) apievents.SCIMCommonData {
	result := apievents.SCIMCommonData{
		Integration:  pluginName,
		ResourceType: resourceType,
		Request: &apievents.SCIMRequest{
			ID:            "", // unused as yet
			SourceAddress: req.RemoteAddr,
			UserAgent:     req.Header.Get("User-Agent"),
			Method:        req.Method,
			Path:          req.URL.Path,
		},
	}

	return result
}

func scimNewResourceAuditEvent(pluginName, resourceType string, req *http.Request, eventType, eventCode string) *apievents.SCIMResourceEvent {
	return &apievents.SCIMResourceEvent{
		Metadata: apievents.Metadata{
			Type: eventType,
			Code: eventCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		SCIMCommonData: scimNewEventCommonData(pluginName, resourceType, req),
	}
}

func scimNewListAuditEvent(pluginName, resourceType string, req *http.Request) *apievents.SCIMListingEvent {
	return &apievents.SCIMListingEvent{
		Metadata: apievents.Metadata{
			Type: events.SCIMListingEvent,
			Code: events.SCIMListResourcesSuccessCode,
		},
		Status: apievents.Status{
			Success: true,
		},
		SCIMCommonData: scimNewEventCommonData(pluginName, resourceType, req),
	}
}

func scimEmitResourceEvent(ctx context.Context, emitter apievents.Emitter, event *apievents.SCIMResourceEvent, eventErr error, failureCode string) error {
	// Don't emit an audit event for access denied as it's an way to spam the
	// audit log as a DoS attack.
	if trace.IsAccessDenied(eventErr) {
		return nil
	}
	if eventErr != nil {
		event.Metadata.Code = failureCode
		event.Status.Success = false
		event.Status.Error = eventErr.Error()
	}
	return trace.Wrap(emitter.EmitAuditEvent(ctx, event))
}
