package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/scim/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	oktaplugin "github.com/gravitational/teleport/e/lib/okta/plugin"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
)

func (s *Service) authorize(ctx context.Context, target *pb.RequestTarget) (types.Plugin, error) {
	if err := s.authorizeGRPCRequest(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	plugin, err := s.GetPlugin(ctx, target.GetPluginId(), true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authorizeSCIMRequest(ctx, target, plugin); err != nil {
		return nil, trace.Wrap(err)
	}

	return plugin, nil
}

func (s *Service) authorizeSCIMRequest(ctx context.Context, target *pb.RequestTarget, plugin types.Plugin) error {
	creds, err := oktaplugin.GetStaticCredentials(ctx, s.AccessPoint, plugin.GetCredentials().GetStaticCredentialsRef())
	if err != nil {
		return trace.Wrap(err)
	}

	switch plugin.GetType() {
	case types.PluginTypeOkta:
		return trace.Wrap(s.authorizeOktaPlugin(creds, target))
	case types.PluginTypeSCIM:
		return trace.Wrap(s.authorizeSCIMPlugin(ctx, plugin, creds, target))

	default:
		return trace.BadParameter("unsupported plugin type %q for SCIM request", plugin.GetType())
	}
}

func (s *Service) authorizeOktaPlugin(creds []types.PluginStaticCredentials, target *pb.RequestTarget) error {
	scimTokenHash, ok, err := oktaplugin.SelectSCIMTokenHash(creds)
	if err != nil {
		return trace.Wrap(err)
	}
	if !ok {
		return trace.AccessDenied("no token set")
	}
	return validateBearerToken(scimTokenHash, target.GetAuthorization())
}

func (s *Service) authorizeSCIMPlugin(ctx context.Context, plugin types.Plugin, creds []types.PluginStaticCredentials, target *pb.RequestTarget) error {
	if len(creds) != 1 {
		return trace.AccessDenied("expected exactly one static credential for SCIM plugin, got %d", len(creds))
	}

	cred, ok := creds[0].(*types.PluginStaticCredentialsV1)
	if !ok {
		return trace.AccessDenied("expected static credential for SCIM plugin, got %T", creds[0])
	}

	switch t := cred.Spec.Credentials.(type) {
	case *types.PluginStaticCredentialsSpecV1_APIToken:
		return trace.Wrap(validateBearerToken(t.APIToken, target.GetAuthorization()))
	case *types.PluginStaticCredentialsSpecV1_OAuthClientSecret:
		return trace.Wrap(s.authorizeOAuthCredentials(ctx, target, plugin))
	default:
		return trace.AccessDenied("unsupported static credential type %T for SCIM plugin", cred.Spec.Credentials)
	}
}

func validateBearerToken(hash, authHeader string) error {
	if hash == "" {
		return trace.AccessDenied("no token set")
	}

	bearerToken, err := extractBearerToken(authHeader)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(bearerToken)); err != nil {
		return trace.AccessDenied("invalid token")
	}
	return nil
}

func extractBearerToken(authHeader string) (string, error) {
	const prefix = "bearer"
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != prefix {
		return "", trace.AccessDenied("malformed bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

// authorizeGRPCRequest checks that the service that is forwarding SCIM request via gRPC
// has Teleport Proxy role and a correct entitlement license is enabled for this cluster.
func (s *Service) authorizeGRPCRequest(ctx context.Context) error {
	if !s.Config.Modules.Features().GetEntitlement(entitlements.OktaSCIM).Enabled {
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

func (s *Service) authorizeOAuthCredentials(ctx context.Context, target *pb.RequestTarget, plugin types.Plugin) error {
	// refLabel is the label of the static credentials reference and is not considered as sensitive information.
	// It is used as a pointer to the specific static credentials used for the plugin.
	refLabel := plugin.GetCredentials().GetStaticCredentialsRef().Labels[eteleport.PluginLabel]
	if refLabel == "" {
		return trace.AccessDenied("plugin %q does not have static credentials defined", plugin.GetName())
	}

	ca, err := s.Config.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: s.Config.ClusterName,
	}, false)
	if err != nil {
		return trace.Wrap(err)
	}

	bearerToken, err := extractBearerToken(target.GetAuthorization())
	if err != nil {
		return trace.Wrap(err)
	}

	for _, keyPair := range ca.GetTrustedJWTKeyPairs() {
		pk, err := keys.ParsePublicKey(keyPair.PublicKey)
		if err != nil {
			return trace.Wrap(err, "failed to parse public key %q", keyPair.PublicKey)
		}

		verifier, err := jwt.New(&jwt.Config{
			Clock:       s.Clock,
			PublicKey:   pk,
			ClusterName: s.Config.ClusterName,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = verifier.VerifyPluginToken(bearerToken, jwt.PluginTokenParam{
			Issuer:   s.ClusterName,
			Audience: []string{"teleport.plugin"},
			Subject:  fmt.Sprintf("plugin:%s:%s", plugin.GetName(), refLabel),
		})
		if err == nil {
			return nil // Authorized
		}
	}

	return trace.AccessDenied("invalid token")
}
