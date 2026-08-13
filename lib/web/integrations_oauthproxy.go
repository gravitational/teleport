package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/zitadel/oidc/v3/pkg/client"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/observability/otelhttp"
	"github.com/gravitational/teleport/lib/utils"
)

const oauthProxyCookieName = "oauthproxy_state"

type oauthProxyCookiePayload struct {
	State    string `json:"state"`
	Verifier string `json:"verifier"`
}

func oauthProxyRedirectURI(publicAddr, integration string) string {
	if !strings.HasPrefix(publicAddr, "https://") {
		publicAddr = "https://" + publicAddr
	}
	return fmt.Sprintf("%s/v1/webapi/oauthproxy/%s/callback", publicAddr, integration)
}

func (h *Handler) oauthProxyAuthorize(w http.ResponseWriter, r *http.Request, p httprouter.Params, sessionCtx *SessionContext) (any, error) {
	integrationName := p.ByName("integration")
	if integrationName == "" {
		return nil, trace.BadParameter("integration name is required")
	}

	clt, err := sessionCtx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ig, err := clt.GetIntegration(r.Context(), integrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if ig.GetSubKind() != types.IntegrationSubKindOAuthProxy {
		return nil, trace.BadParameter("invalid integration subkind %q", ig.GetSubKind())
	}

	spec := ig.GetOAuthProxyIntegrationSpec()
	if spec.ClientId == "" || spec.Issuer == "" || len(spec.Scopes) == 0 {
		return nil, trace.BadParameter("bad spec")
	}

	timeoutCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	dc, err := client.Discover(timeoutCtx, spec.Issuer, otelhttp.DefaultClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	u, err := url.Parse(dc.AuthorizationEndpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := utils.CryptoRandomHex(32)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier, err := utils.CryptoRandomHex(32)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	cookieID, err := utils.CryptoRandomHex(32)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := json.Marshal(oauthProxyCookiePayload{State: state, Verifier: verifier})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     fmt.Sprintf("%s_%s", oauthProxyCookieName, cookieID),
		Value:    base64.RawURLEncoding.EncodeToString(payload),
		Path:     "/v1/webapi/oauthproxy",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   300,
	})

	q := u.Query()
	q.Set("client_id", spec.ClientId)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(spec.Scopes, " "))
	q.Set("redirect_uri", oauthProxyRedirectURI(h.PublicProxyAddr(), integrationName))
	q.Set("state", state+"_"+cookieID)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")

	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusFound)
	return nil, nil
}
