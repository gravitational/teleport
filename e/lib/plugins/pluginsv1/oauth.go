package pluginsv1

import (
	"context"
	"crypto/subtle"
	fmt "fmt"
	"time"

	"github.com/gravitational/trace"

	pluginspb "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/services"
)

// validatePluginType ensures the plugin supports OAuth.
func validatePluginType(t types.PluginType) error {
	if t != types.PluginTypeSCIM {
		return trace.AccessDenied("plugin type %q does not support OAuth", t)
	}
	return nil
}

// validateOauthTokenRequest ensures required fields are set and valid.
func validateOauthTokenRequest(req *pluginspb.CreatePluginOauthTokenRequest) error {
	switch {
	case req.GetPluginName() == "":
		return trace.BadParameter("plugin name cannot be empty")
	case req.GetClientId() == "":
		return trace.BadParameter("client_id cannot be empty")
	case req.GetClientSecret() == "":
		return trace.BadParameter("client_secret cannot be empty")
	case req.GetGrantType() != "client_credentials":
		return trace.BadParameter("unsupported grant_type %q, only 'client_credentials' is supported", req.GetGrantType())
	}
	return nil
}

// CreatePluginOauthToken handles generation of an OAuth token for a plugin.
func (s *Service) CreatePluginOauthToken(ctx context.Context, req *pluginspb.CreatePluginOauthTokenRequest) (*pluginspb.CreatePluginOauthTokenResponse, error) {
	authCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)) {
		return nil, trace.AccessDenied("only proxy role can create plugin oauth tokens")
	}
	if err := validateOauthTokenRequest(req); err != nil {
		return nil, trace.Wrap(err)
	}
	plugin, err := s.getPlugin(ctx, req.GetPluginName())
	if err != nil {
		return nil, err
	}
	if err := validatePluginType(plugin.GetType()); err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := s.getStaticCredentials(ctx, plugin, req.GetPluginName())
	if err != nil {
		return nil, err
	}
	if !s.credentialsMatch(creds, req.GetClientId(), req.GetClientSecret()) {
		return nil, trace.AccessDenied("invalid client_id or client_secret for plugin %q", req.GetPluginName())
	}
	tokenResp, err := s.generatePluginOAuthToken(ctx, plugin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tokenResp, nil
}

// getPlugin retrieves a plugin by name, ensuring it exists.
func (s *Service) getPlugin(ctx context.Context, pluginName string) (types.Plugin, error) {
	plugin, err := s.pluginService.GetPlugin(ctx, pluginName, true)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("plugin %q not found", pluginName)
		}
		return nil, trace.Wrap(err)
	}
	return plugin, nil
}

// getStaticCredentials retrieves static credentials by plugin labels.
func (s *Service) getStaticCredentials(ctx context.Context, plugin types.Plugin, pluginName string) ([]types.PluginStaticCredentials, error) {
	ref := plugin.GetCredentials().GetStaticCredentialsRef()
	if len(ref.Labels) == 0 {
		return nil, trace.AccessDenied("plugin %q does not have static credentials defined", pluginName)
	}
	creds, err := s.pluginStaticCredentialsService.GetPluginStaticCredentialsByLabels(ctx, ref.Labels)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no static credentials found for plugin %q", pluginName)
		}
		return nil, trace.Wrap(err)
	}
	if len(creds) == 0 {
		return nil, trace.AccessDenied("no credentials defined for plugin %q", pluginName)
	}
	return creds, nil
}

// credentialsMatch verifies client_id and client_secret against known credentials.
func (s *Service) credentialsMatch(creds []types.PluginStaticCredentials, clientID, clientSecret string) bool {
	for _, cred := range creds {
		storedID, storedSecret := cred.GetOAuthClientSecret()
		if constantEquals(storedID, clientID) && constantEquals(storedSecret, clientSecret) {
			return true
		}
	}
	return false
}

// constantEquals compares two strings in constant time to prevent timing attacks.
func constantEquals(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// generateAWSOIDCToken signs and returns an OIDC token for the given plugin.
func (s *Service) generatePluginOAuthToken(ctx context.Context, plugin types.Plugin) (*pluginspb.CreatePluginOauthTokenResponse, error) {
	clusterName, err := s.authServer.GetClusterName(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ca, err := s.authServer.GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.OIDCIdPCA,
		DomainName: clusterName.GetClusterName(),
	}, true /* loadKeys */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signer, err := s.keyStoreManager.GetJWTSigner(ctx, ca)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := services.GetJWTSigner(signer, ca.GetClusterName(), s.authServer.GetClock())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pluginCredentialsRef, ok := plugin.GetCredentials().GetStaticCredentialsRef().Labels[eteleport.PluginLabel]
	if !ok || pluginCredentialsRef == "" {
		return nil, trace.AccessDenied("plugin %q does not have static credentials defined", plugin.GetName())
	}

	now := s.authServer.GetClock().Now()
	// Set the expiration time to 1h.
	// The SCIM operation are quire rare need for long-lived tokens.
	expiresAt := now.Add(time.Hour)
	signedToken, err := privateKey.SignPluginToken(jwt.PluginTokenParam{
		Issuer:   clusterName.GetClusterName(),
		Audience: []string{"teleport.plugin"},
		Subject:  fmt.Sprintf("plugin:%s:%s", plugin.GetName(), pluginCredentialsRef),
		Expires:  expiresAt,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pluginspb.CreatePluginOauthTokenResponse_builder{
		AccessToken: signedToken,
		ExpiresIn:   int64(expiresAt.Sub(now) / time.Second),
		TokenType:   "Bearer",
	}.Build(), nil
}
