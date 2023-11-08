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
	"github.com/mitchellh/mapstructure"
	"time"
)

// IDTokenClaims
// See the following for the structure:
// https://docs.spacelift.io/integrations/cloud-providers/oidc/#standard-claims
type IDTokenClaims struct {
	// Sub provides some information about the Spacelift run that generated this
	// token.
	// space:<space_id>:(stack|module):<stack_id|module_id>:run_type:<run_type>:scope:<read|write>
	Sub string `json:"sub"`
	// SpaceID is the ID of the space in which the run that owns the token was
	// executed.
	SpaceID string `json:"spaceId"`
	// CallerType is the type of the caller, ie. the entity that owns the run -
	// either stack or module.
	CallerType string `json:"callerType"`
	// CallerID is the ID of the caller, ie. the stack or module that generated
	// the run.
	CallerID string `json:"callerId"`
	// RunType is the type of the run.
	// (PROPOSED, TRACKED, TASK, TESTING or DESTROY)
	RunType string `json:"runType"`
	// RunID is the ID of the run that owns the token.
	RunID string `json:"runId"`
	// Scope is the scope of the token - either read or write.
	Scope string `json:"scope"`
}

// JoinAuditAttributes returns a series of attributes that can be inserted into
// audit events related to a specific join.
func (c *IDTokenClaims) JoinAuditAttributes() (map[string]interface{}, error) {
	res := map[string]interface{}{}
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &res,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := d.Decode(c); err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

type envGetter func(key string) string

// IDTokenSource allows a SpaceLift ID token to be fetched whilst within a
// SpaceLift execution.
type IDTokenSource struct {
	getEnv envGetter
}

func (its *IDTokenSource) GetIDToken() (string, error) {
	tok := its.getEnv("SPACELIFT_OIDC_TOKEN")
	if tok == "" {
		return "", trace.BadParameter(
			"SPACELIFT_OIDC_TOKEN environment variable missing",
		)
	}

	return tok, nil
}

func NewIDTokenSource(getEnv envGetter) *IDTokenSource {
	return &IDTokenSource{
		getEnv,
	}
}

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
) (*IDTokenValidator, error) {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &IDTokenValidator{
		IDTokenValidatorConfig: cfg,
	}, nil
}

func (id *IDTokenValidator) issuerURL(domain string) string {
	scheme := "https"
	if id.insecure {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s", scheme, domain)
}

func (id *IDTokenValidator) Validate(
	ctx context.Context, domain string, token string,
) (*IDTokenClaims, error) {
	p, err := oidc.NewProvider(
		ctx,
		id.issuerURL(domain),
	)
	if err != nil {
		return nil, trace.Wrap(err, "creating oidc provider")
	}

	verifier := p.Verifier(&oidc.Config{
		// Spacelift uses an audience of your Spacelift tenant hostname
		// This is weird - but we just have to work with this.
		ClientID: domain,
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
