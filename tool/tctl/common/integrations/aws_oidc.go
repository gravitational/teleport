/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package integrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	integrationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/integration/v1"
)

// testAWSOIDC validates that an AWS OIDC integration can assume the configured
// role and reach AWS STS. The assumed role does not require permissions beyond
// the trust relationship with `sts:AssumeRoleWithWebIdentity`.
func (c *Command) testAWSOIDC(ctx context.Context, client awsOIDCPinger) (*testAWSOIDCOutput, error) {
	resp, err := client.Ping(ctx, integrationv1.PingRequest_builder{
		Integration: c.testArgs.integration,
	}.Build())
	if err != nil {
		if trace.IsNotImplemented(err) {
			return nil, trace.NotImplemented("the server does not support testing AWS OIDC integrations")
		}
		return nil, trace.Wrap(err)
	}

	o := &testAWSOIDCOutput{
		Status:         "healthy",
		Integration:    c.testArgs.integration,
		AccountID:      resp.GetAccountId(),
		AssumedRoleARN: resp.GetArn(),
		UserID:         resp.GetUserId(),
	}

	return o, nil
}

type testAWSOIDCOutput struct {
	Status         string `json:"status"`
	Integration    string `json:"integration_name"`
	AccountID      string `json:"account_id"`
	AssumedRoleARN string `json:"assumed_role_arn"`
	UserID         string `json:"user_id"`
}

// String implements fmt.Stringer.
func (o *testAWSOIDCOutput) String() string {
	var sb strings.Builder

	sb.WriteString("AWS STS get-caller-identity response:\n\n")

	sb.WriteString("Integration Name: ")
	sb.WriteString(o.Integration)
	sb.WriteString("\n")
	sb.WriteString("AWS Account ID:   ")
	sb.WriteString(o.AccountID)
	sb.WriteString("\n")
	sb.WriteString("Assumed Role ARN: ")
	sb.WriteString(o.AssumedRoleARN)
	sb.WriteString("\n")
	sb.WriteString("User ID:          ")
	sb.WriteString(o.UserID)
	sb.WriteString("\n")

	sb.WriteString("\n")
	sb.WriteString(bold("AWS OIDC integration is healthy."))
	sb.WriteString("\n")

	return sb.String()
}

type awsOIDCPinger interface {
	Ping(context.Context, *integrationv1.PingRequest, ...grpc.CallOption) (*integrationv1.PingResponse, error)
}

var _ fmt.Stringer = &testAWSOIDCOutput{}
