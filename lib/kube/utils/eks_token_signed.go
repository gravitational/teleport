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
	"encoding/base64"
	"time"

	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/aws/aws-sdk-go/service/sts/stsiface"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// GenAWSEKSToken creates an AWS token to access EKS clusters.
// Logic from https://github.com/aws/aws-cli/blob/6c0d168f0b44136fc6175c57c090d4b115437ad1/awscli/customizations/eks/get_token.py#L211-L229
func GenAWSEKSToken(stsClient stsiface.STSAPI, clusterID string, clock clockwork.Clock) (string, time.Time, error) {
	const (
		// The sts GetCallerIdentity request is valid for 15 minutes regardless of this parameters value after it has been
		// signed.
		requestPresignParam = 60
		// The actual token expiration (presigned STS urls are valid for 15 minutes after timestamp in x-amz-date).
		presignedURLExpiration = 15 * time.Minute
		v1Prefix               = "k8s-aws-v1."
		clusterIDHeader        = "x-k8s-aws-id"
	)

	// generate an sts:GetCallerIdentity request and add our custom cluster ID header
	request, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	request.HTTPRequest.Header.Add(clusterIDHeader, clusterID)

	// Sign the request.  The expires parameter (sets the x-amz-expires header) is
	// currently ignored by STS, and the token expires 15 minutes after the x-amz-date
	// timestamp regardless.  We set it to 60 seconds for backwards compatibility (the
	// parameter is a required argument to Presign(), and authenticators 0.3.0 and older are expecting a value between
	// 0 and 60 on the server side).
	// https://github.com/aws/aws-sdk-go/issues/2167
	presignedURLString, err := request.Presign(requestPresignParam)
	if err != nil {
		return "", time.Time{}, trace.Wrap(err)
	}

	// Set token expiration to 1 minute before the presigned URL expires for some cushion
	tokenExpiration := clock.Now().Add(presignedURLExpiration - 1*time.Minute)
	return v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString)), tokenExpiration, nil
}
