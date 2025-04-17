// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package decision

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/userloginstate"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
)

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

// AccessPoint represents the upstream data source required by the decision service.
type AccessPoint interface {
	services.ClusterNameGetter
	services.RoleGetter
	NodeGetter
	services.AuthPreferenceGetter
	services.AuthorityGetter
	ClusterNetworkingConfigGetter
	services.UserGetter
}

// ULSGenerator is a service that generates user login state without side-effects.
type ULSGenerator interface {
	// GeneratePureULS is a special variant of user login state generation that does not have side-effects
	// and does not consider non-static configuration.
	GeneratePureULS(context.Context, types.User) (*userloginstate.UserLoginState, error)
}

// Config configures the core decision service impl.
type Config struct {
	// AccessPoint is the upstream data source required by the decision service.
	AccessPoint AccessPoint

	// ULSGenerator is the user login state generator required for dry run identity generation.
	ULSGenerator ULSGenerator
}

// CheckAndSetDefaults checks the config and sets default values where appropriate.
func (c *Config) CheckAndSetDefaults() error {
	if c.AccessPoint == nil {
		return trace.BadParameter("decision service config missing required parameter AccessPoint")
	}
	if c.ULSGenerator == nil {
		return trace.BadParameter("decision service config missing required parameter ULSGenerator")
	}
	return nil
}

// Service is the core decision service implementation.
type Service struct {
	cfg Config
}

// NewService creates a new decision service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{
		cfg: cfg,
	}, nil
}

// EvaluateSSHAccess evaluates an SSH access attempt.
//
// XXX: This method is a work in progress prototype. Decisions are not authoritative and should not be used for any
// enforcement-related logic. The contents of this method do not necessarily reflect recommended practices for
// ssh access evaluation and are subject to change.
func (s *Service) EvaluateSSHAccess(ctx context.Context, req *decisionpb.EvaluateSSHAccessRequest) (*decisionpb.EvaluateSSHAccessResponse, error) {
	// check required fields & basic format
	if err := checkEvaluateSSHAccessRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Metadata.DryRun {
		if opts := req.Metadata.DryRunOptions; opts != nil {
			// dry-run requests may omit a true caller identity in favor of specifying a user for which a
			// hypothetical identity should be generated.
			if opts.GenerateIdentity != nil {
				generatedIdent, err := s.GenerateDryRunSSHIdentity(ctx, opts.GenerateIdentity)
				if err != nil {
					return nil, trace.Wrap(err)
				}

				req.SshIdentity = generatedIdent
			}
		}
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
		return &decisionpb.EvaluateSSHAccessResponse{
			Decision: &decisionpb.EvaluateSSHAccessResponse_Denial{
				Denial: &decisionpb.SSHAccessDenial{
					Metadata: &decisionpb.DenialMetadata{
						PdpVersion: teleport.Version,
						UserMessage: fmt.Sprintf("user %s@%s is not authorized to login as %v@%s: %v",
							ident.Username, authority.GetClusterName(), req.OsUser, localClusterName, err),
					},
				},
			},
		}, nil
	}

	// load net config (used during calculation of client idle timeout)
	netConfig, err := s.cfg.AccessPoint.GetClusterNetworkingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// load auth preference (used during calculation of locking mode)
	authPref, err := s.cfg.AccessPoint.GetAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	permit := &decisionpb.SSHAccessPermit{
		Metadata: &decisionpb.PermitMetadata{
			PdpVersion: teleport.Version,
		},
		ForwardAgent:         accessChecker.CheckAgentForward(req.OsUser) == nil,
		X11Forwarding:        accessChecker.PermitX11Forwarding(),
		SshFileCopy:          accessChecker.CanCopyFiles(),
		PortForwardMode:      accessChecker.SSHPortForwardMode(),
		ClientIdleTimeout:    durationFromGoDuration(accessChecker.AdjustClientIdleTimeout(netConfig.GetClientIdleTimeout())),
		SessionRecordingMode: string(accessChecker.SessionRecordingMode(constants.SessionRecordingServiceSSH)),
		LockingMode:          string(accessChecker.LockingMode(authPref.GetLockingMode())),
		// TODO: a *lot* more needs to go here
	}

	return &decisionpb.EvaluateSSHAccessResponse{
		Decision: &decisionpb.EvaluateSSHAccessResponse_Permit{
			Permit: permit,
		},
	}, nil
}

func (s *Service) getLocalClusterName(ctx context.Context) (string, error) {
	clusterName, err := s.cfg.AccessPoint.GetClusterName(ctx)
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
	if localClusterName == ca.GetClusterName() {
		return services.AccessInfoFromLocalSSHIdentity(ident), nil
	}

	return services.AccessInfoFromRemoteSSHIdentity(ident, ca.CombinedMapping())
}

func checkEvaluateSSHAccessRequest(req *decisionpb.EvaluateSSHAccessRequest) error {
	if err := checkSSHIdentityBasedRequest(req); err != nil {
		return trace.Wrap(err)
	}
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

type sshIdentityBasedRequest interface {
	GetMetadata() *decisionpb.RequestMetadata
	GetSshIdentity() *decisionpb.SSHIdentity
}

func checkSSHIdentityBasedRequest(req sshIdentityBasedRequest) error {
	meta := req.GetMetadata()
	if meta == nil {
		return trace.BadParameter("missing required parameter Metadata")
	}

	if meta.DryRun {
		// ensure that the dry run either specifies identity generation or an explicit identity but not both
		if opts := meta.DryRunOptions; opts != nil && opts.GenerateIdentity != nil {
			if req.GetSshIdentity() != nil {
				return trace.BadParameter("cannot specify both SshIdentity and Metadata.DryRunOptions.GenerateIdentity")
			}

			if opts.GenerateIdentity.Username == "" {
				return trace.BadParameter("missing required parameter Username in Metadata.DryRunOptions.GenerateIdentity")
			}
		} else {
			if req.GetSshIdentity() == nil {
				return trace.BadParameter("missing required parameter SshIdentity")
			}

			if err := checkSSHIdentity(req.GetSshIdentity()); err != nil {
				return trace.Wrap(err)
			}
		}
	} else {
		// ensure that standard request specifies an identity and *not* any dry run options
		if req.GetSshIdentity() == nil {
			return trace.BadParameter("missing required parameter SshIdentity")
		}

		if err := checkSSHIdentity(req.GetSshIdentity()); err != nil {
			return trace.Wrap(err)
		}

		if meta.DryRunOptions != nil {
			return trace.BadParameter("unexpected parameter Metadata.DryRunOptions in non-dry-run request")
		}
	}

	return nil
}

func checkSSHIdentity(ident *decisionpb.SSHIdentity) error {
	if ident.CertType != ssh.UserCert {
		return trace.BadParameter("unsupported cert type for ssh identity (%d), expected type 'user' (%d)", ident.CertType, ssh.UserCert)
	}

	return nil
}

func durationFromGoDuration(d time.Duration) *durationpb.Duration {
	if d == 0 {
		return nil
	}
	return durationpb.New(d)
}
