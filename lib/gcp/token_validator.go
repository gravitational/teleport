/*
Copyright 2023 Gravitational, Inc.

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

package gcp

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/jwt"
)

// IDTokenValidatorConfig is the config for IDTokenValidator.
type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
	// issuerHost is the host of the Issuer for tokens issued by Google, to be
	// overridden in tests. Defaults to "accounts.google.com".
	issuerHost string
	// insecure configures the validator to use HTTP rather than HTTPS, to be
	// overridden in tests.
	insecure bool
}

// IDTokenValidator validates ID tokens from GCP.
type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(cfg IDTokenValidatorConfig) *IDTokenValidator {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.issuerHost == "" {
		cfg.issuerHost = defaultIssuerHost
	}
	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

func (id *IDTokenValidator) issuerURL() string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, id.issuerHost)
}

// Validate validates an ID token.
func (id *IDTokenValidator) Validate(ctx context.Context, token string) (*IDTokenClaims, error) {
	p, err := oidc.NewProvider(ctx, id.issuerURL())
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
	if err := jwt.CheckNotBefore(id.Clock.Now(), time.Minute*2, idToken); err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
