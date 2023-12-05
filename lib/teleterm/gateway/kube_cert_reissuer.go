/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
