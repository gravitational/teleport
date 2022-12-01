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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/stretchr/testify/require"

	cloudtest "github.com/gravitational/teleport/lib/cloud/test"
)

func TestReadableResourceName(t *testing.T) {
	t.Parallel()

	type TypeA struct {
	}

	tests := []struct {
		name          string
		inputResource interface{}
		expectOutput  string
	}{
		{
			name:          "Redshift Serverless workgroup",
			inputResource: cloudtest.RedshiftServerlessWorkgroup("my-workgroup", ""),
			expectOutput:  `Redshift Serverless workgroup "my-workgroup" (namespace "my-namespace")`,
		},
		{
			name: "S3 bucket",
			inputResource: &s3.Bucket{
				Name: aws.String("my-bucket"),
			},
			expectOutput: `S3 Bucket "my-bucket"`,
		},
		{
			name:          "unknown name",
			inputResource: &TypeA{},
			expectOutput:  `Aws TypeA "<unknown>"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectOutput, ReadableResourceName(test.inputResource))
		})
	}
}
