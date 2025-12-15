/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package pgcommon

import (
	"context"
	"crypto/tls"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/certreloader"
)

// CreateClientCertReloader creates a client reloader for postgres compatible connections. The
// connString is expected to be in a url format.
func CreateClientCertReloader(ctx context.Context, name, connString string, reloadInterval time.Duration, expiry prometheus.Gauge) (func(*tls.CertificateRequestInfo) (*tls.Certificate, error), error) {
	u, err := url.Parse(connString)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "conn_string must be in url format when reload interval is set")
	}
	vals := u.Query()

	reloader := certreloader.New(certreloader.Config{
		KeyPairsWithMetric: []certreloader.KeyPairWithMetric{
			{
				KeyPairPath: servicecfg.KeyPairPath{
					PrivateKey:  vals.Get("sslkey"),
					Certificate: vals.Get("sslcert"),
				},
				Expiry: expiry,
			},
		},
		KeyPairsReloadInterval: reloadInterval,
	}, name, teleport.ComponentAuth)
	if err := reloader.Run(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	return reloader.GetClientCertificate, nil
}
