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

package aws

import (
	"context"
	"net/http"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/aws/aws-sdk-go/aws"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func Test_convertSTSError(t *testing.T) {
	tests := []struct {
		name        string
		input       error
		wantErrorAs interface{}
	}{
		{
			name:        "no error",
			input:       nil,
			wantErrorAs: nil,
		},
		{
			name:        "IDPCommunicationErrorException",
			input:       &ststypes.IDPCommunicationErrorException{},
			wantErrorAs: new(trace.ConnectionProblemError),
		},
		{
			name: "no permission to assume role",
			input: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{Response: &http.Response{
						StatusCode: http.StatusForbidden,
					}},
					Err: trace.Errorf("User: arn:aws:sts::123456789012:assumed-role/alice/i-00112233445566778 is not authorized to perform: sts:AssumeRole on resource: arn:aws:iam::123456789012:role/bob"),
				},
			},
			wantErrorAs: new(trace.AccessDeniedError),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := convertSTSError(test.input)
			if test.wantErrorAs == nil {
				require.NoError(t, err)
			} else {
				require.ErrorAs(t, err, &test.wantErrorAs)
			}
		})
	}
}

func Test_stsEndpointResolver(t *testing.T) {
	resolver := newSTSEndpointResolver()
	tests := []struct {
		name         string
		params       sts.EndpointParameters
		wantEndpoint string
	}{
		{
			name:         "global (no region)",
			params:       sts.EndpointParameters{},
			wantEndpoint: "https://sts.amazonaws.com",
		},
		{
			name: "ca-central-1",
			params: sts.EndpointParameters{
				Region: aws.String("ca-central-1"),
			},
			wantEndpoint: "https://sts.ca-central-1.amazonaws.com",
		},
		{
			name: "us-west-2 fips",
			params: sts.EndpointParameters{
				Region:  aws.String("us-west-2"),
				UseFIPS: aws.Bool(true),
			},
			wantEndpoint: "https://sts-fips.us-west-2.amazonaws.com",
		},
		{
			name: "us-gov-east-1 fips",
			params: sts.EndpointParameters{
				Region:  aws.String("us-gov-east-1"),
				UseFIPS: aws.Bool(true),
			},
			wantEndpoint: "https://sts.us-gov-east-1.amazonaws.com",
		},
		{
			name: "cn-northwest-1",
			params: sts.EndpointParameters{
				Region: aws.String("cn-northwest-1"),
			},
			wantEndpoint: "https://sts.cn-northwest-1.amazonaws.com.cn",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			endpoint, err := resolver.ResolveEndpoint(context.Background(), test.params)
			require.NoError(t, err)
			require.Equal(t, test.wantEndpoint, endpoint.URI.String())
		})
	}
}
