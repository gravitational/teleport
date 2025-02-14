package decision

import (
	"context"

	decisionpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/decision/v1alpha1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
	"github.com/gravitational/trace"
)

func (s *Service) GenerateDryRunSSHIdentity(ctx context.Context, req *decisionpb.DryRunIdentity) (*decisionpb.SSHIdentity, error) {
	// XXX: modeled heavily off of auth/methods.go login logic. this is for use with dry-run requests only
	// and is *not* suitable for making any binding access decisions fo any kind.

	// get the core state user configuration
	user, err := s.cfg.AccessPoint.GetUser(ctx, req.Username, false /* withSecrets */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// generate a user login state value for the user (this will apply dynamic configuration such as access lists)
	userState, err := s.cfg.ULSGenerator.GeneratePureULS(ctx, user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessInfo := services.AccessInfoFromUserState(userState)

	localClusterName, err := s.getLocalClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	checker, err := services.NewAccessChecker(accessInfo, localClusterName, s.cfg.AccessPoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	allowedLogins, err := checker.CheckLoginDuration(0 /* all logins regardless of ttl */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return SSHIdentityFromSSHCA(&sshca.Identity{
		Username:              user.GetName(),
		Principals:            allowedLogins,
		Roles:                 checker.RoleNames(),
		PermitPortForwarding:  checker.CanPortForward(),
		PermitAgentForwarding: checker.CanForwardAgents(),
		PermitX11Forwarding:   checker.PermitX11Forwarding(),
		Traits:                userState.GetTraits(),
		CertificateExtensions: checker.CertificateExtensions(),
	}), nil
}
