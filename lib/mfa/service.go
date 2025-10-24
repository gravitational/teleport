package mfa

import (
	"context"
	"log/slog"

	"github.com/gravitational/teleport/api/client/proto"
	mfav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/mfa/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
)

// CheckAndSetDefaults checks the config and sets default values where appropriate.
func (c *Config) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("mfa service config missing required parameter AccessPoint")
	}

	return nil
}

// NodeGetter is a service that gets a node.
type NodeGetter interface {
	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
}

// ClusterNetworkingConfigGetter is a service that gets the cluster networking configuration.
type ClusterNetworkingConfigGetter interface {
	// GetClusterNetworkingConfig returns the cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
}

type MFADeviceGetter interface {
	// GetMFADevices returns the MFA devices for a user.
	GetMFADevices(ctx context.Context, user string, withSecrets bool) ([]*types.MFADevice, error)
}

// AccessPoint represents the upstream data source required by the mfa service.
type AccessPoint interface {
	services.ClusterNameGetter
	services.RoleGetter
	NodeGetter
	services.AuthPreferenceGetter
	services.AuthorityGetter
	ClusterNetworkingConfigGetter
	services.UserGetter
}

// Config configures the core mfa service impl.
type Config struct {
	// AccessPoint is the upstream data source required by the mfa service.
	AccessPoint AccessPoint

	// MFADeviceGetter is used to retrieve MFA devices.
	MFADeviceGetter MFADeviceGetter

	// Identity is used to manage WebAuthn session data.
	Identity wanlib.LoginIdentity

	Emitter apievents.Emitter
}

// Service is the core mfa service implementation.
type Service struct {
	cfg Config
}

var _ mfav1.MFAServiceServer = &Service{}

// NewService creates a new mfa service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		cfg: cfg,
	}, nil
}

// CreateChallengeForAction creates an MFA challenge that is tied to a specific user action. The action_id is required
// and the created challenge will be correlated to that action.
func (s *Service) CreateChallengeForAction(
	ctx context.Context,
	req *mfav1.CreateChallengeForActionRequest,
) (*mfav1.CreateChallengeForActionResponse, error) {
	switch {
	case req.GetActionId() == "":
		return nil, trace.BadParameter("action_id is required")

	case req.GetUser() == "":
		return nil, trace.BadParameter("user is required")
	}

	slog.Debug("Creating MFA challenge for action", slog.String("action_id", req.GetActionId()))

	username, err := authz.GetClientUsername(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	challenges, err := s.mfaAuthChallenge(ctx, username, req.GetSsoClientRedirectUrl(), req.GetProxyAddress(), req.GetActionId())
	if err != nil {
		return nil, trace.AccessDenied("unable to create MFA challenges")
	}

	mfaChal := &mfav1.MFAAuthenticateChallenge{}
	mfaChal.WebauthnChallenge = challenges.GetWebauthnChallenge()
	mfaChal.SsoChallenge = (*mfav1.SSOChallenge)(challenges.GetSSOChallenge())

	if mfaChal.WebauthnChallenge == nil && mfaChal.SsoChallenge == nil {
		return nil, trace.BadParameter("no MFA challenges could be created for user %q", req.GetUser())
	}

	slog.Debug("Created MFA challenge for action", slog.String("action_id", req.GetActionId()))

	// Convert to the response type.
	return &mfav1.CreateChallengeForActionResponse{
		ActionId:     req.GetActionId(),
		MfaChallenge: mfaChal,
	}, nil
}

// mfaAuthChallenge constructs an MFAAuthenticateChallenge for all MFA devices registered by the user.
// TODO(cthach): This function needs to be updated to persist the action ID with each challenge created.
func (a *Service) mfaAuthChallenge(ctx context.Context, user, ssoClientRedirectURL, proxyAddress string, actionID string) (*proto.MFAAuthenticateChallenge, error) {
	// Check what kind of MFA is enabled.
	apref, err := a.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	enableWebauthn := apref.IsSecondFactorWebauthnAllowed()
	// enableSSO := apref.IsSecondFactorSSOAllowed()

	// Fetch configurations. The IsSecondFactor*Allowed calls above already
	// include the necessary checks of config empty, disabled, etc.
	var u2fPref *types.U2F
	switch val, err := apref.GetU2F(); {
	case trace.IsNotFound(err): // OK, may happen.
	case err != nil: // NOK, unexpected.
		return nil, trace.Wrap(err)
	default:
		u2fPref = val
	}
	var webConfig *types.Webauthn
	switch val, err := apref.GetWebauthn(); {
	case trace.IsNotFound(err): // OK, may happen.
	case err != nil: // NOK, unexpected.
		return nil, trace.Wrap(err)
	default:
		webConfig = val
	}

	// User required for non-passwordless.
	if user == "" {
		return nil, trace.BadParameter("user required")
	}

	devs, err := a.cfg.MFADeviceGetter.GetMFADevices(ctx, user, true /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	groupedDevs := groupByDeviceType(devs)
	challenge := &proto.MFAAuthenticateChallenge{}

	slog.Debug("User has MFA devices",
		slog.String("user", user),
		slog.Bool("totp", groupedDevs.TOTP),
		slog.Int("webauthn_count", len(groupedDevs.Webauthn)),
		slog.Bool("sso", groupedDevs.SSO != nil),
		slog.Bool("webauthn_enabled", enableWebauthn),
	)

	// WebAuthn challenge.
	if enableWebauthn && len(groupedDevs.Webauthn) > 0 {
		slog.Debug("Creating WebAuthn MFA challenge for action", slog.String("action_id", actionID))

		webLogin := &wanlib.LoginFlow{
			U2F:      u2fPref,
			Webauthn: webConfig,
			Identity: wanlib.WithDevices(a.cfg.Identity, groupedDevs.Webauthn),
		}
		assertion, err := webLogin.Begin(
			ctx,
			user,
			&mfav1.ChallengeExtensions{
				Scope: mfav1.ChallengeScope_CHALLENGE_SCOPE_USER_SESSION,
			},
			&actionID,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		slog.Debug("Created WebAuthn MFA challenge for action", slog.String("action_id", actionID))

		challenge.WebauthnChallenge = wantypes.CredentialAssertionToProto(assertion)
	}

	// TODO(cthach): Implement SSO MFA challenge.
	// // If the user has an SSO device and the client provided a redirect URL to handle
	// // the MFA SSO flow, create an SSO challenge.
	// if enableSSO && groupedDevs.SSO != nil && ssoClientRedirectURL != "" {
	// 	if challenge.SSOChallenge, err = a.beginSSOMFAChallenge(ctx, user, groupedDevs.SSO.GetSso(), ssoClientRedirectURL, proxyAddress, challengeExtensions); err != nil {
	// 		return nil, trace.Wrap(err)
	// 	}
	// }

	clusterName, err := a.cfg.AccessPoint.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO(cthach): Fix UI is showing "unknown" for this event.
	if err := a.cfg.Emitter.EmitAuditEvent(ctx, &apievents.CreateMFAChallengeForAction{
		Metadata: apievents.Metadata{
			Type:        events.CreateMFAChallengeForActionEvent,
			Code:        events.CreateMFAChallengeForActionCode,
			ClusterName: clusterName.GetClusterName(),
		},
		UserMetadata: authz.ClientUserMetadataWithUser(ctx, user),
		ActionID:     actionID,
	}); err != nil {
		slog.WarnContext(ctx, "Failed to emit event", "error", err)
	}

	return challenge, nil
}

type devicesByType struct {
	TOTP     bool
	Webauthn []*types.MFADevice
	SSO      *types.MFADevice
}

func groupByDeviceType(devs []*types.MFADevice) devicesByType {
	res := devicesByType{}
	for _, dev := range devs {
		switch dev.Device.(type) {
		case *types.MFADevice_Totp:
			res.TOTP = true
		case *types.MFADevice_U2F:
			res.Webauthn = append(res.Webauthn, dev)
		case *types.MFADevice_Webauthn:
			res.Webauthn = append(res.Webauthn, dev)
		case *types.MFADevice_Sso:
			res.SSO = dev
		default:
			slog.WarnContext(context.Background(), "Skipping MFA device with unknown type", "device_type", logutils.TypeAttr(dev.Device))
		}
	}
	return res
}

// ValidateChallengeForAction validates the MFA challenge response provided by the user for a specific user action.
// The action_id is required and must match the action the challenge was created for.
func (s *Service) ValidateChallengeForAction(
	ctx context.Context,
	req *mfav1.ValidateChallengeForActionRequest,
) (*mfav1.ValidateChallengeForActionResponse, error) {
	switch {
	case req.GetActionId() == "":
		return nil, trace.BadParameter("action_id is required")

	case req.GetMfaResponse() == nil:
		return nil, trace.BadParameter("response is required")

	case req.GetUser() == "":
		return nil, trace.BadParameter("user is required")
	}

	slog.Debug("Validating MFA challenge for action", slog.String("action_id", req.GetActionId()))

	return nil, trace.NotImplemented("not implemented")
}
