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

package cirleci

import (
	"context"
	"encoding/json"
	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"time"
)

func ValidateToken(ctx context.Context, clock clockwork.Clock, organizationID string, token string) (*IDTokenClaims, error) {
	issuer := IssuerURL(organizationID)
	// We can't cache the provider as the issuer changes for every circleci
	// organization - we should build a provider that can support a global
	// cache.
	p, err := oidc.NewProvider(
		context.Background(),
		issuer,
	)

	verifier := p.Verifier(&oidc.Config{
		ClientID: organizationID,
		Now:      clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// `go-oidc` does not implement not before check, so we need to manually
	// perform this
	if err := checkNotBefore(clock.Now(), time.Minute*2, idToken); err != nil {
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
