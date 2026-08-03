package clientiprestrictionv1

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	clientiprestrictionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/clientiprestriction/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	cloudv1 "github.com/gravitational/teleport/e/api/cloud/v1"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/modules"
)

var errCloudClientNotReady = trace.Errorf("unable to communicate with the Teleport Cloud API")

// Valid values for the ClientIPRestriction spec.mode field. An empty mode is
// treated as enforced for backward compatibility.
const (
	clientIPRestrictionModeDraft    = "draft"
	clientIPRestrictionModeEnforced = "enforced"
)

// CloudClientGetter retrieves a client for interacting with Teleport Cloud.
type CloudClientGetter interface {
	GetCloudClient() cloudv1.TenantsServiceClient
}

// ServiceConfig holds configuration options for the ClientIPRestriction gRPC service.
type ServiceConfig struct {
	Authorizer        authz.Authorizer
	Emitter           apievents.Emitter
	Modules           modules.Modules
	Logger            *slog.Logger
	CloudClientGetter CloudClientGetter
}

// Service implements the gRPC API layer for the singleton ClientIPRestriction resource.
type Service struct {
	clientiprestrictionv1pb.UnimplementedClientIPRestrictionServiceServer

	authorizer        authz.Authorizer
	emitter           apievents.Emitter
	cloudClientGetter CloudClientGetter
	modules           modules.Modules
	logger            *slog.Logger
}

// NewService returns a new Service.
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("Authorizer is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("Emitter is required")
	case cfg.CloudClientGetter == nil:
		return nil, trace.BadParameter("CloudClientGetter is required")
	case cfg.Modules == nil:
		return nil, trace.BadParameter("Modules is required")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("Logger is required")
	}
	return &Service{
		authorizer:        cfg.Authorizer,
		emitter:           cfg.Emitter,
		cloudClientGetter: cfg.CloudClientGetter,
		modules:           cfg.Modules,
		logger:            cfg.Logger,
	}, nil
}

func (s *Service) checkEntitlement() error {
	if s.modules.Features().Entitlements[entitlements.ClientIPRestrictions].Enabled {
		return nil
	}
	return trace.AccessDenied("client_ip_restriction resources require the ClientIPRestrictions entitlement - please contact support@goteleport.com")
}

func (s *Service) authorizeUser(ctx context.Context) (*authz.Context, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.IsLocalOrRemoteUser(*authCtx) {
		return nil, trace.ConnectionProblem(nil, "client_ip_restriction API is only available to users")
	}
	return authCtx, nil
}

func (s *Service) cloudClient() (cloudv1.TenantsServiceClient, error) {
	c := s.cloudClientGetter.GetCloudClient()
	if c == nil {
		return nil, errCloudClientNotReady
	}
	return c, nil
}

// GetClientIPRestriction returns the ClientIPRestriction singleton.
func (s *Service) GetClientIPRestriction(ctx context.Context, req *clientiprestrictionv1pb.GetClientIPRestrictionRequest) (*clientiprestrictionv1pb.GetClientIPRestrictionResponse, error) {
	if name := req.GetName(); name != "" && name != types.MetaNameClientIPRestriction {
		return nil, trace.BadParameter("client_ip_restriction name must be %q or empty, got %q", types.MetaNameClientIPRestriction, name)
	}
	if err := s.checkEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindClientIPRestriction, types.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := s.cloudClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := cc.GetClientIPRestriction(ctx, &cloudv1.GetClientIPRestrictionRequest{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clientiprestrictionv1pb.GetClientIPRestrictionResponse_builder{
		ClientIpRestriction: fromCloud(resp.GetClientIpRestriction()),
	}.Build(), nil
}

// CreateClientIPRestriction creates a new ClientIPRestriction.
func (s *Service) CreateClientIPRestriction(ctx context.Context, req *clientiprestrictionv1pb.CreateClientIPRestrictionRequest) (_ *clientiprestrictionv1pb.CreateClientIPRestrictionResponse, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer s.emitMutationEvent(ctx, authCtx, req.GetClientIpRestriction(), &err)

	if err := s.checkEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindClientIPRestriction, types.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateClientIPRestriction(req.GetClientIpRestriction()); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := s.cloudClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := cc.CreateClientIPRestriction(ctx, &cloudv1.CreateClientIPRestrictionRequest{
		Cidrs:   req.GetClientIpRestriction().GetSpec().GetAllowedCidrs(),
		Mode:    modeToCloud(req.GetClientIpRestriction().GetSpec().GetMode()),
		Expires: req.GetClientIpRestriction().GetSpec().GetExpires(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clientiprestrictionv1pb.CreateClientIPRestrictionResponse_builder{
		ClientIpRestriction: fromCloud(resp.GetClientIpRestriction()),
	}.Build(), nil
}

// UpdateClientIPRestriction updates an existing ClientIPRestriction.
func (s *Service) UpdateClientIPRestriction(ctx context.Context, req *clientiprestrictionv1pb.UpdateClientIPRestrictionRequest) (_ *clientiprestrictionv1pb.UpdateClientIPRestrictionResponse, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer s.emitMutationEvent(ctx, authCtx, req.GetClientIpRestriction(), &err)

	if err := s.checkEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindClientIPRestriction, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateClientIPRestriction(req.GetClientIpRestriction()); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := s.cloudClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := cc.UpdateClientIPRestriction(ctx, &cloudv1.UpdateClientIPRestrictionRequest{
		Cidrs:    req.GetClientIpRestriction().GetSpec().GetAllowedCidrs(),
		Revision: req.GetClientIpRestriction().GetMetadata().GetRevision(),
		Mode:     modeToCloud(req.GetClientIpRestriction().GetSpec().GetMode()),
		Expires:  req.GetClientIpRestriction().GetSpec().GetExpires(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clientiprestrictionv1pb.UpdateClientIPRestrictionResponse_builder{
		ClientIpRestriction: fromCloud(resp.GetClientIpRestriction()),
	}.Build(), nil
}

// UpsertClientIPRestriction creates or replaces the ClientIPRestriction singleton.
func (s *Service) UpsertClientIPRestriction(ctx context.Context, req *clientiprestrictionv1pb.UpsertClientIPRestrictionRequest) (_ *clientiprestrictionv1pb.UpsertClientIPRestrictionResponse, err error) {
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer s.emitMutationEvent(ctx, authCtx, req.GetClientIpRestriction(), &err)

	if err := s.checkEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindClientIPRestriction, types.VerbCreate, types.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := validateClientIPRestriction(req.GetClientIpRestriction()); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := s.cloudClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resp, err := cc.UpsertClientIPRestriction(ctx, &cloudv1.UpsertClientIPRestrictionRequest{
		Cidrs:   req.GetClientIpRestriction().GetSpec().GetAllowedCidrs(),
		Mode:    modeToCloud(req.GetClientIpRestriction().GetSpec().GetMode()),
		Expires: req.GetClientIpRestriction().GetSpec().GetExpires(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clientiprestrictionv1pb.UpsertClientIPRestrictionResponse_builder{
		ClientIpRestriction: fromCloud(resp.GetClientIpRestriction()),
	}.Build(), nil
}

// DeleteClientIPRestriction deletes the ClientIPRestriction singleton.
func (s *Service) DeleteClientIPRestriction(ctx context.Context, req *clientiprestrictionv1pb.DeleteClientIPRestrictionRequest) (resp *clientiprestrictionv1pb.DeleteClientIPRestrictionResponse, err error) {
	if name := req.GetName(); name != "" && name != types.MetaNameClientIPRestriction {
		return nil, trace.BadParameter("client_ip_restriction name must be %q or empty, got %q", types.MetaNameClientIPRestriction, name)
	}
	authCtx, err := s.authorizeUser(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
		return nil, trace.Wrap(err)
	}
	defer s.emitMutationEvent(ctx, authCtx, nil, &err)

	if err := s.checkEntitlement(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := authCtx.CheckAccessToKind(types.KindClientIPRestriction, types.VerbDelete); err != nil {
		return nil, trace.Wrap(err)
	}

	cc, err := s.cloudClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := cc.DeleteClientIPRestriction(ctx, &cloudv1.DeleteClientIPRestrictionRequest{}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &clientiprestrictionv1pb.DeleteClientIPRestrictionResponse{}, nil
}

// emitMutationEvent emits a ClientIPRestrictionsUpdate audit event for any mutating RPC.
// Intended to be called via defer so it captures the final error value.
func (s *Service) emitMutationEvent(ctx context.Context, authCtx *authz.Context, cir *clientiprestrictionv1pb.ClientIPRestriction, errPtr *error) {
	code := events.ClientIPRestrictionsUpdateCode
	var errMsg string
	if *errPtr != nil {
		errMsg = (*errPtr).Error()
	}

	var cidrs []string
	var mode string
	var enforcementExpires time.Time
	if *errPtr == nil && cir != nil {
		cidrs = cir.GetSpec().GetAllowedCidrs()
		// Record mode and expiry exactly as accepted by the Cloud API, with no
		// client-side normalization: an empty mode and a zero expiry mean the
		// request did not set them.
		mode = cir.GetSpec().GetMode()
		if expires := cir.GetSpec().GetExpires(); expires != nil {
			enforcementExpires = expires.AsTime()
		}
	}

	userMetadata := authz.ClientUserMetadata(ctx)
	if emitErr := s.emitter.EmitAuditEvent(ctx, &apievents.ClientIPRestrictionsUpdate{
		Metadata: apievents.Metadata{
			Type: events.ClientIPRestrictionsUpdateEvent,
			Code: code,
		},
		UserMetadata: userMetadata,
		ResourceMetadata: apievents.ResourceMetadata{
			Name:      types.KindClientIPRestriction,
			UpdatedBy: userMetadata.User,
		},
		ConnectionMetadata:   authz.ConnectionMetadata(ctx),
		ClientIPRestrictions: cidrs,
		Mode:                 mode,
		EnforcementExpires:   enforcementExpires,
		Status: apievents.Status{
			Success: *errPtr == nil,
			Error:   errMsg,
		},
	}); emitErr != nil {
		s.logger.WarnContext(ctx, "Failed to emit audit event", "error", emitErr)
	}
}

// fromCloud converts a Cloud API ClientIPRestriction to the Teleport proto type.
func fromCloud(in *cloudv1.ClientIPRestriction) *clientiprestrictionv1pb.ClientIPRestriction {
	if in == nil {
		return clientiprestrictionv1pb.ClientIPRestriction_builder{
			Kind:    types.KindClientIPRestriction,
			Version: types.V1,
			Metadata: headerv1.Metadata_builder{
				Name: types.MetaNameClientIPRestriction,
			}.Build(),
			Spec:   &clientiprestrictionv1pb.ClientIPRestrictionSpec{},
			Status: &clientiprestrictionv1pb.ClientIPRestrictionStatus{},
		}.Build()
	}

	return clientiprestrictionv1pb.ClientIPRestriction_builder{
		Kind:    types.KindClientIPRestriction,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:     types.MetaNameClientIPRestriction,
			Revision: in.GetRevision(),
		}.Build(),
		Spec: clientiprestrictionv1pb.ClientIPRestrictionSpec_builder{
			AllowedCidrs: in.GetCidrs(),
			Mode:         cloudModeToString(in.GetMode()),
			Expires:      in.GetExpires(),
		}.Build(),
		Status: clientiprestrictionv1pb.ClientIPRestrictionStatus_builder{
			State: cloudStatusToString(in.GetStatus()),
		}.Build(),
	}.Build()
}

// validateClientIPRestriction checks that a ClientIPRestriction resource is well-formed
// before sending it to the Cloud API. Only name and revision are supported on metadata;
// the name must be empty or equal to MetaNameClientIPRestriction.
func validateClientIPRestriction(cir *clientiprestrictionv1pb.ClientIPRestriction) error {
	if cir == nil {
		return trace.BadParameter("client_ip_restriction is required")
	}
	md := cir.GetMetadata()
	if name := md.GetName(); name != "" && name != types.MetaNameClientIPRestriction {
		return trace.BadParameter("client_ip_restriction name must be %q or empty, got %q", types.MetaNameClientIPRestriction, name)
	}
	switch {
	case md.GetDescription() != "",
		md.GetExpires() != nil,
		len(md.GetLabels()) > 0,
		md.GetNamespace() != "":
		return trace.BadParameter("only name and revision fields are supported on metadata for client_ip_restriction resources")
	}
	switch mode := cir.GetSpec().GetMode(); mode {
	case "", clientIPRestrictionModeDraft, clientIPRestrictionModeEnforced:
	default:
		return trace.BadParameter("client_ip_restriction mode must be %q, %q, or empty, got %q", clientIPRestrictionModeDraft, clientIPRestrictionModeEnforced, mode)
	}
	return nil
}

// cloudStatusToString converts the Cloud API ClientIPRestrictionStatus enum to the
// string value used in the Teleport proto status field.
func cloudStatusToString(s cloudv1.ClientIPRestrictionStatus) string {
	switch s {
	case cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_PENDING:
		return "pending"
	case cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_ACTIVE:
		return "active"
	case cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_DRAFT:
		return "draft"
	case cloudv1.ClientIPRestrictionStatus_CLIENT_IP_RESTRICTION_STATUS_EXPIRED:
		return "expired"
	default:
		return "unknown"
	}
}

// modeToCloud converts the Teleport proto spec.mode string to the Cloud API
// ClientIPRestrictionMode enum. An empty mode maps to unspecified, which the
// Cloud API treats as enforced for backward compatibility.
func modeToCloud(mode string) cloudv1.ClientIPRestrictionMode {
	switch mode {
	case clientIPRestrictionModeDraft:
		return cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT
	case clientIPRestrictionModeEnforced:
		return cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED
	default:
		return cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_UNSPECIFIED
	}
}

// cloudModeToString converts the Cloud API ClientIPRestrictionMode enum to the
// string value used in the Teleport proto spec.mode field. Unspecified maps to
// an empty string so the value round-trips cleanly.
func cloudModeToString(m cloudv1.ClientIPRestrictionMode) string {
	switch m {
	case cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_DRAFT:
		return clientIPRestrictionModeDraft
	case cloudv1.ClientIPRestrictionMode_CLIENT_IP_RESTRICTION_MODE_ENFORCED:
		return clientIPRestrictionModeEnforced
	default:
		return ""
	}
}
