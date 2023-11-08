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

package spacelift

import (
	"context"
	"fmt"
	"github.com/coreos/go-oidc"
	"github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"time"
)

type IDTokenValidatorConfig struct {
	// Clock is used by the validator when checking expiry and issuer times of
	// tokens. If omitted, a real clock will be used.
	Clock clockwork.Clock
	// insecure configures the validator to use HTTP rather than HTTPS. This
	// is not exported as this is only used in the test for now.
	insecure bool
}

type IDTokenValidator struct {
	IDTokenValidatorConfig
}

func NewIDTokenValidator(
	cfg IDTokenValidatorConfig,
) *IDTokenValidator {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}
}

func (id *IDTokenValidator) issuerURL(hostname string) string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, hostname)
}

func (id *IDTokenValidator) Validate(
	ctx context.Context, hostname string, token string,
) (*IDTokenClaims, error) {
	p, err := oidc.NewProvider(
		ctx,
		id.issuerURL(hostname),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating oidc provider")
	}

	verifier := p.Verifier(&oidc.Config{
		// Spacelift uses an audience of your Spacelift tenant hostname
		// This is weird - but we just have to work with this.
		ClientID: hostname,
		Now:      id.Clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err, "verifying token")
	}

	// `go-oidc` does not implement not before check, so we need to manually
	// perform this
	if err := jwt.CheckNotBefore(
		id.Clock.Now(), time.Minute*2, idToken,
	); err != nil {
		return nil, trace.Wrap(err, "enforcing nbf")
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
