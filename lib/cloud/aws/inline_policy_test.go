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

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestInlinePolicy(t *testing.T) {
	tests := []struct {
		name             string
		inputPolicyName  string
		inputIdentityARN string
		inputIAM         iamiface.IAMAPI
		expectNewError   bool
	}{
		{
			name:             "unknown identity",
			inputPolicyName:  "policy-name",
			inputIdentityARN: "arn:aws:iam::1234567890:group/readers",
			expectNewError:   true,
		},
		{
			name:             "inline policy for IAM role",
			inputPolicyName:  "policy-name",
			inputIdentityARN: "arn:aws:iam::1234567890:role/some-role",
			inputIAM:         &iamMock{},
		},
		{
			name:             "inline policy for IAM user",
			inputPolicyName:  "policy-name",
			inputIdentityARN: "arn:aws:iam::1234567890:user/some-user",
			inputIAM:         &iamMock{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			identity, err := IdentityFromArn(test.inputIdentityARN)
			require.NoError(t, err)

			inlinePolicy, err := NewInlinePolicyForIdentity(test.inputPolicyName, test.inputIAM, identity)
			if test.expectNewError {
				require.Error(t, err)
				return
			}

			ctx := context.TODO()
			putDocument := NewPolicyDocument()
			err = inlinePolicy.Put(ctx, putDocument)
			require.NoError(t, err)

			getDocument, err := inlinePolicy.Get(ctx)
			require.NoError(t, err)
			require.Equal(t, putDocument, getDocument)

			err = inlinePolicy.Delete(ctx)
			require.NoError(t, err)

			_, err = inlinePolicy.Get(ctx)
			require.True(t, trace.IsNotFound(err), "expect error is trace.NotFound")
		})
	}
}
