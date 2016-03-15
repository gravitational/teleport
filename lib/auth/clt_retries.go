/*
Copyright 2015 Gravitational, Inc.

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

package auth

import (
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
)

type limitedClient interface {
	GetNodes() ([]services.Server, error)
	GetCertAuthorities(caType services.CertAuthType) ([]*services.CertAuthority, error)
}

type retryingClient struct {
	limitedClient
	retries int
}

func RetryingClient(client limitedClient, retries int) *retryingClient {
	if retries < 1 {
		retries = 1
	}
	return &retryingClient{
		limitedClient: client,
		retries:       retries,
	}
}

func (c *retryingClient) GetServers() ([]services.Server, error) {
	var e error
	for i := 0; i < c.retries; i++ {
		servers, err := c.limitedClient.GetNodes()
		if err == nil {
			return servers, nil
		}
		e = err
	}
	return nil, trace.Wrap(e)
}

func (c *retryingClient) GetCertAuthorities(caType services.CertAuthType) ([]*services.CertAuthority, error) {
	var e error
	for i := 0; i < c.retries; i++ {
		cas, err := c.limitedClient.GetCertAuthorities(caType)
		if err == nil {
			return cas, nil
		}
		e = err
	}
	return nil, trace.Wrap(e)
}
