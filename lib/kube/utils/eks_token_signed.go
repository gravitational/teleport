/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package utils

import (
	"context"
	"encoding/base64"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// STSPresignClient is the subset of the STS presign client we need to generate EKS tokens.
type STSPresignClient interface {
	PresignGetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// GenAWSEKSToken creates an AWS token to access EKS clusters.
// Logic from https://github.com/aws/aws-cli/blob/6c0d168f0b44136fc6175c57c090d4b115437ad1/awscli/customizations/eks/get_token.py#L211-L229
func GenAWSEKSToken(ctx context.Context, stsClient STSPresignClient, clusterID string, clock clockwork.Clock) (string, time.Time, error) {
	const (
		// The actual token expiration (presigned STS urls are valid for 15 minutes after timestamp in x-amz-date).
		presignedURLExpiration = 15 * time.Minute
		v1Prefix               = "k8s-aws-v1."
		clusterIDHeader        = "x-k8s-aws-id"
	)

	presignedReq, err := stsClient.PresignGetCallerIdentity(ctx, nil, func(po *sts.PresignOptions) {
		po.ClientOptions = append(po.ClientOptions, func(o *sts.Options) {
			o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
				return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("ClusterIDHeaderMW", func(
					ctx context.Context, input middleware.FinalizeInput, next middleware.FinalizeHandler,
				) (middleware.FinalizeOutput, middleware.Metadata, error) {
					req, ok := input.Request.(*smithyhttp.Request)
					if ok {
						req.Header.Add(clusterIDHeader, clusterID)
					}
					return next.HandleFinalize(ctx, input)
				}), middleware.After)
			})
		})
	})
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	// Set token expiration to 1 minute before the presigned URL expires for some cushion.
	tokenExpiration := clock.Now().Add(presignedURLExpiration - 1*time.Minute)
	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedReq.URL)), tokenExpiration, nil
}
