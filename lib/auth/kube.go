/*
Copyright 2018 Gravitational, Inc.

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
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/kube/authority"
)

// KubeCSR is a kubernetes CSR request
type KubeCSR struct {
	// CSR is a kubernetes CSR
	CSR []byte `json:"csr"`
}

// CheckAndSetDefaults checks and sets defaults
func (a *KubeCSR) CheckAndSetDefaults() error {
	if len(a.CSR) == 0 {
		return trace.BadParameter("missing parameter 'csr'")
	}
	return nil
}

// KubeCSRREsponse is a response to kubernetes CSR request
type KubeCSRResponse struct {
	// Cert is a signed certificate PEM block
	Cert []byte `json:"cert"`
	// CA is a PEM block with trusted cert authorities
	CA []byte `json:"ca"`
}

// ProcessKubeCSR processes CSR request against Kubernetes CA, returns
// signed certificate if sucessfull.
func (s *AuthServer) ProcessKubeCSR(req KubeCSR) (*KubeCSRResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := authority.ProcessCSR(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &KubeCSRResponse{Cert: cert.Cert, CA: cert.CA}, nil
}
