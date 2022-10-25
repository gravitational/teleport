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

package circleci

import (
	"context"

	"github.com/coreos/go-oidc"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

func ValidateToken(
	ctx context.Context,
	clock clockwork.Clock,
	issuerURLTemplate string,
	organizationID string,
	token string,
) (*IDTokenClaims, error) {
	issuer := issuerURL(issuerURLTemplate, organizationID)
	// We can't cache the provider as the issuer changes for every circleci
	// organization - we should build a provider that can support a global
	// cache.
	p, err := oidc.NewProvider(
		ctx,
		issuer,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	verifier := p.Verifier(&oidc.Config{
		ClientID: organizationID,
		Now:      clock.Now,
	})

	idToken, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	claims := IDTokenClaims{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, trace.Wrap(err)
	}
	return &claims, nil
}
