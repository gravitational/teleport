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

package app

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/sts"
)

// mockCredentialsProvider mocks AWS credentials.Provider interface.
type mockCredentialsProvider struct {
	retrieveValue credentials.Value
	retrieveError error
}

func (m mockCredentialsProvider) Retrieve() (credentials.Value, error) {
	return m.retrieveValue, m.retrieveError
}
func (m mockCredentialsProvider) IsExpired() bool {
	return false
}

// mockAssumeRoler mocks AWS stscreds.AssumeRoler interface.
type mockAssumeRoler struct {
	output *sts.AssumeRoleOutput
}

func (m mockAssumeRoler) AssumeRole(input *sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error) {
	return m.output, nil
}
