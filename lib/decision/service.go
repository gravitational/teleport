package decision

import (
	"context"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
)

// Interface is the core interface for the decision service.
type Interface interface {
	EvaluateSSHAccess(ctx context.Context, req *decisionpb.EvaluateSSHAccessRequest) (*decisionpb.EvaluateSSHAccessResponse, error)
}

// NodeGetter is a service that gets a node.
type NodeGetter interface {
	// GetNode returns a node by name and namespace.
	GetNode(ctx context.Context, namespace, name string) (types.Server, error)
}

type ClusterNetworkingConfigGetter interface {
	// GetClusterNetworkingConfig returns the cluster networking configuration.
	GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error)
}

type ClusterNameGetter interface {
	// GetClusterName gets types.ClusterName from the backend.
	GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error)
}

// AccessPoint represents the upstream data source required by the decision service.
type AccessPoint interface {
	ClusterNameGetter
	services.RoleGetter
	NodeGetter
	services.AuthPreferenceGetter
	services.AuthorityGetter
	ClusterNetworkingConfigGetter
}

// Config configures the core decision service impl.
type Config struct {
	AccessPoint AccessPoint
}

// CheckAndSetDefaults checks the config and sets default values where appropriate.
func (c *Config) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("decision service config missing required parameter AccessPoint")
	}
	return nil
}

type Service struct {
	cfg Config
}

func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		cfg: cfg,
	}, nil
}

func (s *Service) EvaluateSSHAccess(ctx context.Context, req *decisionpb.EvaluateSSHAccessRequest) (*decisionpb.EvaluateSSHAccessResponse, error) {
	// check required fields & basic format
	if err := checkEvaluateSSHAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	ident := SSHIdentityToSSHCA(req.SshIdentity)

	authority, err := s.resolveSSHAuthority(ctx, req.SshAuthority)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localClusterName, err := s.getLocalClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate access info derived from the identity and the authority (may include cross-cluster mapping if authority is from
	// a remote cluster).
	accessInfo, err := buildAccessInfo(ident, authority, localClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessChecker(accessInfo, localClusterName, s.cfg.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := services.AccessStateFromSSHIdentity(ctx, ident, accessChecker, s.cfg.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.OsUser == teleport.SSHSessionJoinPrincipal {
		// XXX: this is the point in the process where ahLoginChecker.canLoginWithRBAC forks into session access
		// evaluation. It is still unclear how we should be handling session-joining within the decision method.
		// For the time being, we will consider it an error, but this must be resolved before this method can
		// by used for real enforcement.
		return nil, trace.Errorf("session joining is not yet supported by the decision service")
	}

	target, err := s.cfg.AccessPoint.GetNode(ctx, apidefaults.Namespace, req.Node.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check if roles allow access to server
	if err := accessChecker.CheckAccess(
		target,
		state,
		services.NewLoginMatcher(req.OsUser),
	); err != nil {
		// XXX: rework this into an explicit denial msg
		return nil, trace.AccessDenied("user %s@%s is not authorized to login as %v@%s: %v",
			ident.Username, authority.GetClusterName(), req.OsUser, localClusterName, err)
	}

	// load net config (used during calculation of client idle timeout)
	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	permit := &decisionpb.SSHAccessPermit{
		// XXX: add metadata to permit
		ForwardAgent:      accessChecker.CheckAgentForward(req.OsUser) == nil,
		X11Forwarding:     accessChecker.PermitX11Forwarding(),
		SshFileCopy:       accessChecker.CanCopyFiles(),
		PortForwardMode:   accessChecker.SSHPortForwardMode(),
		ClientIdleTimeout: durationFromGoDuration(accessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout())),
		// TODO: a *lot* more needs to go here
	}

	return &decisionpb.EvaluateSSHAccessResponse{
		Decision: &decisionpb.EvaluateSSHAccessResponse_Permit{
			Permit: permit,
		},
	}, nil
}

func (s *Service) getLocalClusterName(ctx context.Context) (string, error) {
	clusterName, err := s.cfg.AccessPoint.GetClusterName()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return clusterName.GetClusterName(), nil
}

// resolveSSHAuthority is a helper used to resolve the SSHAuthority reference type from a decision request into a cert authority resource.
func (s *Service) resolveSSHAuthority(ctx context.Context, sshAuthority *decisionpb.SSHAuthority) (types.CertAuthority, error) {
	ca, err := s.cfg.AccessPoint.GetCertAuthority(ctx, types.CertAuthID{
		DomainName: sshAuthority.ClusterName,
		Type:       types.CertAuthType(sshAuthority.AuthorityType),
	}, false)
	return ca, trace.Wrap(err)
}

// buildAccessInfo fetches the services.AccessChecker (after role mapping)
// together with the original roles (prior to role mapping) assigned to a
// Teleport user.
func buildAccessInfo(ident *sshca.Identity, ca types.CertAuthority, localClusterName string) (*services.AccessInfo, error) {
	var accessInfo *services.AccessInfo
	var err error
	if localClusterName == ca.GetClusterName() {
		accessInfo = services.AccessInfoFromLocalSSHIdentity(ident)
	} else {
		accessInfo, err = services.AccessInfoFromRemoteSSHIdentity(ident, ca.CombinedMapping())
	}
	return accessInfo, trace.Wrap(err)
}

func checkEvaluateSSHAccessRequest(req *decisionpb.EvaluateSSHAccessRequest) error {
	if req.SshAuthority == nil {
		return trace.BadParameter("missing required parameter SshAuthority")
	}
	if req.SshAuthority.ClusterName == "" {
		return trace.BadParameter("missing required parameter SshAuthority.ClusterName")
	}
	if req.SshAuthority.AuthorityType == "" {
		return trace.BadParameter("missing required parameter SshAuthority.AuthorityType")
	}
	if types.CertAuthType(req.SshAuthority.AuthorityType) != types.UserCA {
		return trace.BadParameter("unsupported cert authority type %q, expected type %q", req.SshAuthority.AuthorityType, types.UserCA)
	}

	if req.SshIdentity == nil {
		return trace.BadParameter("missing required parameter SshIdentity")
	}
	if req.SshIdentity.CertType != ssh.UserCert {
		return trace.BadParameter("unsupported cert type for ssh access eval (%d), expected type 'user' (%d)", req.SshIdentity.CertType, ssh.UserCert)
	}

	if req.Node == nil {
		return trace.BadParameter("missing required parameter Node")
	}
	if req.Node.Name == "" {
		return trace.BadParameter("missing required parameter Node.Name")
	}
	if req.Node.Kind != "" && req.Node.Kind != types.KindNode {
		return trace.BadParameter("unsupported resource kind for ssh access eval %q, expected %q", req.Node.Kind, types.KindNode)
	}

	if req.OsUser == "" {
		// XXX: remove this requirement once we have login enumeration support
		return trace.BadParameter("missing required parameter OsUser")
	}

	return nil
}

func durationToGoDuration(d *durationpb.Duration) time.Duration {
	// nil or "zero" Timestamps are mapped to Go's zero time (0-0-0 0:0.0) instead
	// of unix epoch. The latter avoids problems with tooling (eg, Terraform) that
	// sets structs to their defaults instead of using nil.
	if d == nil || (d.Seconds == 0 && d.Nanos == 0) {
		return 0
	}
	return d.AsDuration()
}

func durationFromGoDuration(d time.Duration) *durationpb.Duration {
	if d == 0 {
		return nil
	}
	return durationpb.New(d)
}
