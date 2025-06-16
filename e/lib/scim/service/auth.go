package service

import (
	"context"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
)

func (s *Service) authorize(ctx context.Context, target *pb.RequestTarget) (types.Plugin, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	plugin, err := s.PluginsService.GetPlugin(ctx, target.GetPluginId(), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.authorizeSCIMRequest(ctx, target, plugin); err != nil {
		return nil, trace.Wrap(err)
	}
	return plugin, nil
}

func (s *Service) authorizeSCIMRequest(ctx context.Context, target *pb.RequestTarget, plugin types.Plugin) error {
	creds, err := oktaplugin.GetStaticCredentials(ctx, s.CredentialsService, plugin.GetCredentials().GetStaticCredentialsRef())
	if err != nil {
		return trace.Wrap(err)
	}

	var scimTokenHash string
	switch plugin.GetType() {
	case types.PluginTypeOkta:
		// Okta Plugin support multiple credential where scim token is marked by dedicated label
		// To get the scim token creds find a creds that the okta scim purpose label.
		var ok bool
		scimTokenHash, ok, err = oktaplugin.SelectSCIMTokenHash(creds)
		if err != nil {
			return trace.Wrap(err)
		}
		if !ok {
			return trace.AccessDenied("no token set")
		}
	case types.PluginTypeSCIM:
		// SCIM Plugin supports only one static credential, we are sure that if there is a static credential
		// it is the SCIM token credential.
		if len(creds) != 1 {
			return trace.AccessDenied("expected exactly one static credential for SCIM plugin, got %d", len(creds))
		}
		scimTokenHash = creds[0].GetAPIToken()
	default:
		return trace.BadParameter("unsupported plugin type %q for SCIM request", plugin.GetType())
	}

	if err := checkBearerToken(scimTokenHash, target.GetAuthorization()); err != nil {
		return trace.AccessDenied("invalid token")
	}
	return nil
}

func checkBearerToken(hash, authHeader string) error {
	bearerToken, err := extractBearerToken(authHeader)
	if err != nil {
		return trace.Wrap(err)
	}

	expectedHash := []byte(hash)
	actualBits := []byte(bearerToken)
	if err := bcrypt.CompareHashAndPassword(expectedHash, actualBits); err != nil {
		return trace.AccessDenied("invalid token")
	}

	return nil
}

func extractBearerToken(authHeader string) (string, error) {
	const (
		hdrBearer = "bearer"
		errMsg    = "malformed bearer token"
	)

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 {
		return "", trace.BadParameter("%s", errMsg)
	}

	if strings.ToLower(parts[0]) != hdrBearer {
		return "", trace.BadParameter("%s", errMsg)
	}

	return strings.TrimSpace(parts[1]), nil
}

// authorizeGRPCRequest checks that the service that is forwarding SCIM request via gRPC
// has Teleport Proxy role and a correct entitlement license is enabled for this cluster.
func (s *Service) authorizeGRPCRequest(ctx context.Context) error {
	if !modules.GetModules().Features().GetEntitlement(entitlements.OktaSCIM).Enabled {
		return trace.NotImplemented("SCIM support requires Identity Governance license")
	}
	authCtx, err := s.Authorizer.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return trace.AccessDenied("only Teleport Proxy may issue requests to the SCIM service")
	}

	return nil
}
