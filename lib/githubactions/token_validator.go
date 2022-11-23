/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package githubactions

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
	// IssuerURL is the URL of the OIDC token issuer, on which the
	// /well-known/openid-configuration endpoint can be found.
	// If this is omitted, a default value will be set.
	IssuerURL string
}

type IDTokenValidator struct {
	IDTokenValidatorConfig

	mu sync.Mutex
	// oidc is protected by mu and should only be accessed via the getProvider
	// method.
	oidc *oidc.Provider
}

func NewIDTokenValidator(cfg IDTokenValidatorConfig) *IDTokenValidator {
	if cfg.IssuerURL == "" {
		cfg.IssuerURL = IssuerURL
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

// getProvider allows the lazy initialisation of the oidc provider.
func (id *IDTokenValidator) getProvider() (*oidc.Provider, error) {
	id.mu.Lock()
	defer id.mu.Unlock()
	if id.oidc != nil {
		return id.oidc, nil
	}
	// Intentionally use context.Background() here since this actually controls
	// cache functionality.
	p, err := oidc.NewProvider(
		context.Background(),
		id.IDTokenValidatorConfig.IssuerURL,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	id.oidc = p
	return p, nil
}

func (id *IDTokenValidator) Validate(ctx context.Context, token string) (*IDTokenClaims, error) {
	p, err := id.getProvider()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: "teleport.cluster.local",
		Now:      id.Clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// `go-oidc` does not implement not before check, so we need to manually
	// perform this
	if err := checkNotBefore(id.Clock.Now(), time.Minute*2, idToken); err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}

// checkNotBefore ensures the token was not issued in the future.
// https://www.rfc-editor.org/rfc/rfc7519#section-4.1.5
// 4.1.5.  "nbf" (Not Before) Claim
// TODO(strideynet): upstream support for `nbf` into the go-oidc lib.
func checkNotBefore(now time.Time, leeway time.Duration, token *oidc.IDToken) error {
	claims := struct {
		NotBefore *jsonTime `json:"nbf"`
	}{}
	if err := token.Claims(&claims); err != nil {
		return trace.Wrap(err)
	}

	if claims.NotBefore != nil {
		adjustedNow := now.Add(leeway)
		nbf := time.Time(*claims.NotBefore)
		if adjustedNow.Before(nbf) {
			return trace.AccessDenied("token not before in future")
		}
	}

	return nil
}

// jsonTime unmarshaling sourced from https://github.com/gravitational/go-oidc/blob/master/oidc.go#L295
// TODO(strideynet): upstream support for `nbf` into the go-oidc lib.
type jsonTime time.Time

func (j *jsonTime) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	var unix int64

	if t, err := n.Int64(); err == nil {
		unix = t
	} else {
		f, err := n.Float64()
		if err != nil {
			return err
		}
		unix = int64(f)
	}
	*j = jsonTime(time.Unix(unix, 0))
	return nil
}
