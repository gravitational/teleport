package web

import (
	"cmp"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/web"
)

// beamPublishPort is the default port the beam compute service accepts
// when publishing a beam (enforced by lib/services/beam.go).
// It is used when a publish request omits the port.
const beamPublishPort = 8080

// beamDefaultPublishProtocol is the protocol used when a publish request omits the protocol.
const beamDefaultPublishProtocol = beamsv1.Protocol_PROTOCOL_HTTP

// listBeamsResponse is the body returned by the list endpoint.
type listBeamsResponse struct {
	Items         []beamDetails `json:"items"`
	NextPageToken string        `json:"next_page_token"`
}

// beamDetails is the full JSON shape returned by list, get, create and update.
type beamDetails struct {
	Name           string             `json:"name"`
	Alias          string             `json:"alias"`
	User           string             `json:"user"`
	Expires        time.Time          `json:"expires"`
	NodeId         string             `json:"node_id"`
	AppName        string             `json:"app_name"`
	EgressMode     string             `json:"egress_mode"`
	AllowedDomains []string           `json:"allowed_domains"`
	Publish        *beamPublishConfig `json:"publish,omitempty"`
	ComputeStatus  string             `json:"compute_status"`
}

// beamPublishConfig is a beam's publish settings, a non-nil value represents a published beam
// and a nil value represents an unpublished beam. Protocol defaults to "http" and Port to 8080 when omitted.
type beamPublishConfig struct {
	Port     uint32 `json:"port"`
	Protocol string `json:"protocol"`
}

type createBeamRequest struct {
	EgressMode     string   `json:"egress_mode"`
	AllowedDomains []string `json:"allowed_domains"`
}

// registerBeamHandlers wires the web API routes that back the Beams UI.
// All routes are entitlement-gated and require an authenticated cluster session.
func (p *Plugin) registerBeamHandlers() {
	p.h.GET("/webapi/sites/:site/beams", p.h.WithClusterAuth(p.listBeams))
	p.h.POST("/webapi/sites/:site/beams", p.h.WithClusterAuth(p.createBeam))
	p.h.GET("/webapi/sites/:site/beams/:name", p.h.WithClusterAuth(p.getBeam))
	p.h.PUT("/webapi/sites/:site/beams/:name", p.h.WithClusterAuth(p.updateBeam))
	p.h.DELETE("/webapi/sites/:site/beams/:name", p.h.WithClusterAuth(p.deleteBeam))
}

// checkBeamsEntitlement returns AccessDenied if the cluster is not entitled to use Beams.
func (p *Plugin) checkBeamsEntitlement() error {
	if entitlement, ok := p.h.GetClusterFeatures().Entitlements[string(entitlements.Beams)]; !ok || !entitlement.Enabled {
		return trace.AccessDenied("not authorized to use beams")
	}
	return nil
}

// listBeams returns a page of beams visible to the caller.
// Supports page_size, page_token, sort_field, sort_dir and repeatable user query params.
func (p *Plugin) listBeams(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if err := p.checkBeamsEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}

	querystring := r.URL.Query()

	sortField, err := parseBeamsSortField(querystring.Get("sort_field"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sortOrder, err := parseBeamsSortOrder(querystring.Get("sort_dir"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pageSize, err := web.QueryLimitAsInt32(querystring, "page_size", 0)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	request := beamsv1.ListBeamsRequest_builder{
		PageSize:  pageSize,
		PageToken: querystring.Get("page_token"),
		SortField: sortField,
		SortOrder: sortOrder,
		Filters: beamsv1.ListBeamsRequest_Filters_builder{
			Users: querystring["user"],
		}.Build(),
	}.Build()

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	resp, err := lt.BeamServiceClient().ListBeams(r.Context(), request)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	beams := make([]beamDetails, 0, len(resp.GetBeams()))
	for _, b := range resp.GetBeams() {
		beams = append(beams, makeBeamDetails(b))
	}

	return listBeamsResponse{
		Items:         beams,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

// createBeam creates a new beam.
func (p *Plugin) createBeam(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if err := p.checkBeamsEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}

	var req createBeamRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	egress, err := parseBeamEgressMode(req.EgressMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	resp, err := lt.BeamServiceClient().CreateBeam(r.Context(), beamsv1.CreateBeamRequest_builder{
		Egress:         egress,
		AllowedDomains: req.AllowedDomains,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeBeamDetails(resp.GetBeam()), nil
}

// getBeam fetches a single beam by its UUID.
func (p *Plugin) getBeam(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if err := p.checkBeamsEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}

	name := params.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing beam name")
	}

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	resp, err := lt.BeamServiceClient().GetBeam(r.Context(), beamsv1.GetBeamRequest_builder{
		Name: &name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeBeamDetails(resp.GetBeam()), nil
}

// updateBeam applies the mutable subset of fields to an existing beam.
func (p *Plugin) updateBeam(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if err := p.checkBeamsEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}

	name := params.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing beam name")
	}

	var req beamDetails
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	current, err := lt.BeamServiceClient().GetBeam(r.Context(), beamsv1.GetBeamRequest_builder{
		Name: &name,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	target := current.GetBeam()

	egress, err := parseBeamEgressMode(req.EgressMode)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	target.GetSpec().SetEgress(egress)
	target.GetSpec().SetAllowedDomains(req.AllowedDomains)

	// A non-nil publish config publishes the beam; a nil one unpublishes it.
	if req.Publish != nil {
		spec, err := req.Publish.toSpec()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		target.GetSpec().SetPublish(spec)
	} else {
		target.GetSpec().SetPublish(nil)
	}

	resp, err := lt.BeamServiceClient().UpdateBeam(r.Context(), beamsv1.UpdateBeamRequest_builder{
		Beam: target,
	}.Build())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeBeamDetails(resp.GetBeam()), nil
}

// deleteBeam tears down a beam and all of its supporting resources.
func (p *Plugin) deleteBeam(_ http.ResponseWriter, r *http.Request, params httprouter.Params, sctx *web.SessionContext, cluster reversetunnelclient.Cluster) (any, error) {
	if err := p.checkBeamsEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}

	name := params.ByName("name")
	if name == "" {
		return nil, trace.BadParameter("missing beam name")
	}

	lt, err := sctx.GetUserClient(r.Context(), cluster)
	if err != nil {
		return nil, trace.Wrap(err, "getting user client")
	}

	if _, err := lt.BeamServiceClient().DeleteBeam(r.Context(), beamsv1.DeleteBeamRequest_builder{
		Name: name,
	}.Build()); err != nil {
		return nil, trace.Wrap(err)
	}

	return web.OK(), nil
}

// toSpec converts a publish config request into a beam PublishSpec, applying
// defaults for any omitted fields.
func (p beamPublishConfig) toSpec() (*beamsv1.PublishSpec, error) {
	protocol := beamDefaultPublishProtocol
	if p.Protocol != "" {
		parsed, err := parseBeamProtocol(p.Protocol)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		protocol = parsed
	}

	return beamsv1.PublishSpec_builder{
		Port:     cmp.Or(p.Port, beamPublishPort),
		Protocol: protocol,
	}.Build(), nil
}

// parseBeamProtocol parses the publish protocol from the update request body.
func parseBeamProtocol(protocol string) (beamsv1.Protocol, error) {
	switch strings.ToLower(protocol) {
	case "http":
		return beamsv1.Protocol_PROTOCOL_HTTP, nil
	case "tcp":
		return beamsv1.Protocol_PROTOCOL_TCP, nil
	default:
		return beamsv1.Protocol_PROTOCOL_UNSPECIFIED, trace.BadParameter("unsupported protocol %q but expected http or tcp", protocol)
	}
}

// parseBeamEgressMode parses the egress mode from the create request body.
func parseBeamEgressMode(mode string) (beamsv1.EgressMode, error) {
	switch mode {
	case "", "unrestricted":
		return beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED, nil
	case "restricted":
		return beamsv1.EgressMode_EGRESS_MODE_RESTRICTED, nil
	default:
		return beamsv1.EgressMode_EGRESS_MODE_UNSPECIFIED, trace.BadParameter("unsupported egress mode %q but expected unrestricted or restricted", mode)
	}
}

// makeBeamDetails projects a beam proto into the create/get/update JSON shape.
func makeBeamDetails(b *beamsv1.Beam) beamDetails {
	allowedDomains := b.GetSpec().GetAllowedDomains()
	if allowedDomains == nil {
		allowedDomains = []string{}
	}

	out := beamDetails{
		Name:           b.GetMetadata().GetName(),
		Alias:          b.GetStatus().GetAlias(),
		User:           b.GetStatus().GetUser(),
		Expires:        b.GetSpec().GetExpires().AsTime(),
		NodeId:         b.GetStatus().GetNodeId(),
		AppName:        b.GetStatus().GetAppName(),
		EgressMode:     beamEgressModeString(b.GetSpec().GetEgress()),
		AllowedDomains: allowedDomains,
		ComputeStatus:  beamComputeStatusString(b.GetStatus().GetComputeStatus()),
	}
	if pub := b.GetSpec().GetPublish(); pub != nil {
		out.Publish = &beamPublishConfig{
			Port:     pub.GetPort(),
			Protocol: beamProtocolString(pub.GetProtocol()),
		}
	}
	return out
}

// beamEgressModeString renders an EgressMode for JSON responses.
// Stored beams always carry RESTRICTED or UNRESTRICTED
func beamEgressModeString(m beamsv1.EgressMode) string {
	switch m {
	case beamsv1.EgressMode_EGRESS_MODE_UNRESTRICTED:
		return "unrestricted"
	case beamsv1.EgressMode_EGRESS_MODE_RESTRICTED:
		return "restricted"
	default:
		return ""
	}
}

// beamProtocolString renders a Protocol for JSON responses.
func beamProtocolString(p beamsv1.Protocol) string {
	switch p {
	case beamsv1.Protocol_PROTOCOL_HTTP:
		return "http"
	case beamsv1.Protocol_PROTOCOL_TCP:
		return "tcp"
	default:
		return ""
	}
}

// beamComputeStatusString renders a ComputeStatus for JSON responses.
// An empty string is valid and means "server default".
func beamComputeStatusString(s beamsv1.ComputeStatus) string {
	switch s {
	case beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_PENDING:
		return "provision_pending"
	case beamsv1.ComputeStatus_COMPUTE_STATUS_PROVISION_COMPLETE:
		return "provision_complete"
	default:
		return ""
	}
}

// parseBeamsSortField parses the list endpoint's sort_field query parameter.
// An empty string is valid and means "server default".
func parseBeamsSortField(field string) (beamsv1.BeamSortField, error) {
	switch field {
	case "name":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_NAME, nil
	case "alias":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_ALIAS, nil
	case "user":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_USER, nil
	case "expires":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_EXPIRES, nil
	case "":
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_UNSPECIFIED, nil
	default:
		return beamsv1.BeamSortField_BEAM_SORT_FIELD_UNSPECIFIED, trace.BadParameter("unsupported sort field %q but expected name, alias, user or expires", field)
	}
}

// parseBeamsSortOrder parses the list endpoint's sort_dir query parameter.
// Case-insensitive; an empty string is valid and means "server default".
func parseBeamsSortOrder(order string) (beamsv1.BeamSortOrder, error) {
	switch strings.ToLower(order) {
	case "asc":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_ASCENDING, nil
	case "desc":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_DESCENDING, nil
	case "":
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_UNSPECIFIED, nil
	default:
		return beamsv1.BeamSortOrder_BEAM_SORT_ORDER_UNSPECIFIED, trace.BadParameter("unsupported sort order %q but expected asc or desc", order)
	}
}
