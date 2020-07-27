/*
Copyright 2020 Gravitational, Inc.

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

package proxy

import (
	"errors"

	"gopkg.in/check.v1"
	authzapi "k8s.io/api/authorization/v1"
	authztypes "k8s.io/client-go/kubernetes/typed/authorization/v1"
)

type AuthSuite struct{}

var _ = check.Suite(AuthSuite{})

func (s AuthSuite) TestCheckImpersonationPermissions(c *check.C) {
	tests := []struct {
		desc             string
		sarErr           error
		allowedVerbs     []string
		allowedResources []string

		wantErr bool
	}{
		{
			desc:    "request failure",
			sarErr:  errors.New("uh oh"),
			wantErr: true,
		},
		{
			desc:             "all permissions granted",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users", "groups", "serviceaccounts"},
			wantErr:          false,
		},
		{
			desc:             "missing some permissions",
			allowedVerbs:     []string{"impersonate"},
			allowedResources: []string{"users"},
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		c.Log(tt.desc)
		mock := &mockSARClient{
			err:              tt.sarErr,
			allowedVerbs:     tt.allowedVerbs,
			allowedResources: tt.allowedResources,
		}
		err := checkImpersonationPermissions(mock)
		if tt.wantErr {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}

type mockSARClient struct {
	authztypes.SelfSubjectAccessReviewInterface

	err              error
	allowedVerbs     []string
	allowedResources []string
}

func (c *mockSARClient) Create(sar *authzapi.SelfSubjectAccessReview) (*authzapi.SelfSubjectAccessReview, error) {
	if c.err != nil {
		return nil, c.err
	}

	var verbAllowed, resourceAllowed bool
	for _, v := range c.allowedVerbs {
		if v == sar.Spec.ResourceAttributes.Verb {
			verbAllowed = true
			break
		}
	}
	for _, r := range c.allowedResources {
		if r == sar.Spec.ResourceAttributes.Resource {
			resourceAllowed = true
			break
		}
	}

	sar.Status.Allowed = verbAllowed && resourceAllowed
	return sar, nil
}
