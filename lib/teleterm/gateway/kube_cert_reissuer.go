/*
Copyright 2023 Gravitational, Inc.

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

package gateway

import (
	"context"
	"crypto/tls"
	"sync/atomic"

	"github.com/gravitational/trace"
)

// kubeCertReissuer implements a simple single-kube cert reissuer that can be
// used for kube local proxy middleware.
type kubeCertReissuer struct {
	cert          atomic.Value
	onExpiredCert func(context.Context) error
}

func newKubeCertReissuer(cert tls.Certificate, onExpiredCert func(context.Context) error) *kubeCertReissuer {
	r := &kubeCertReissuer{
		onExpiredCert: onExpiredCert,
	}
	r.updateCert(cert)
	return r
}

// reissueCert implements alpnproxy.KubeCertReissuer. Arguments
// "teleportCluster" and "kubeCluster" are omitted as this reissuer is bound to
// a single kube cluster.
func (r *kubeCertReissuer) reissueCert(ctx context.Context, _, _ string) (tls.Certificate, error) {
	if err := r.onExpiredCert(ctx); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return r.cert.Load().(tls.Certificate), nil
}

func (r *kubeCertReissuer) updateCert(cert tls.Certificate) error {
	r.cert.Store(cert)
	return nil
}
