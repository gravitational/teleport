package appaccess

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/web"
	"github.com/stretchr/testify/require"
)

func verifyJWT(t *testing.T, pack *Pack, token, appURI string) {
	// Get and unmarshal JWKs
	status, body, err := pack.MakeRequest("", http.MethodGet, "/.well-known/jwks.json")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	var jwks web.JWKSResponse
	err = json.Unmarshal([]byte(body), &jwks)
	require.NoError(t, err)
	require.Len(t, jwks.Keys, 1)
	publicKey, err := jwt.UnmarshalJWK(jwks.Keys[0])
	require.NoError(t, err)

	// Verify JWT.
	key, err := jwt.New(&jwt.Config{
		PublicKey:   publicKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: pack.jwtAppClusterName,
	})
	require.NoError(t, err)
	claims, err := key.Verify(jwt.VerifyParams{
		Username: pack.username,
		RawToken: token,
		URI:      appURI,
	})
	require.NoError(t, err)
	require.Equal(t, pack.username, claims.Username)
	require.Equal(t, pack.user.GetRoles(), claims.Roles)
}
