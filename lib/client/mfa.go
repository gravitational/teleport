/*
Copyright 2021 Gravitational, Inc.

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

package client

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/client/mfa"
)

// promptMFAChallenge is used to mock PromptMFAChallenge for tests.
var promptMFAChallenge = mfa.PromptMFAChallenge

// PromptMFAChallenge prompts the user to complete MFA authentication
// challenges.
// If proxyAddr is empty, the TeleportClient.WebProxyAddr is used.
// See client.PromptMFAChallenge.
func (tc *TeleportClient) PromptMFAChallenge(ctx context.Context, proxyAddr string, c *proto.MFAAuthenticateChallenge, applyOpts func(opts *mfa.PromptMFAChallengeOpts)) (*proto.MFAAuthenticateResponse, error) {
	ctx, span := tc.Tracer.Start(
		ctx,
		"teleportClient/PromptMFAChallenge",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
		oteltrace.WithAttributes(
			attribute.String("cluster", tc.SiteName),
			attribute.Bool("prefer_otp", tc.PreferOTP),
		),
	)
	defer span.End()

	addr := proxyAddr
	if addr == "" {
		addr = tc.WebProxyAddr
	}

	opts := &mfa.PromptMFAChallengeOpts{
		AuthenticatorAttachment: tc.AuthenticatorAttachment,
		PreferOTP:               tc.PreferOTP,
	}
	if applyOpts != nil {
		applyOpts(opts)
	}

	return promptMFAChallenge(ctx, c, addr, opts)
}
