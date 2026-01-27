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
	"crypto/x509"
	"log/slog"
	"net/url"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils/certreloader"
)

// CreateClientCertReloader creates a client reloader for postgres compatible connections. The
// connString is expected to be in a url format. This updates the connConfig to use the created reloader
func CreateClientCertReloader(ctx context.Context, name, connString string, connConfig *pgx.ConnConfig, reloadInterval time.Duration, expiry prometheus.Gauge) error {
	u, err := url.Parse(connString)
	if err != nil {
		return trace.Wrap(err, "the connection string must be in url format when a reload interval is set")
	}
	vals := u.Query()

	var callback func(string, *x509.Certificate)
	if expiry != nil {
		callback = func(path string, cert *x509.Certificate) {
			expiry.Set(float64(cert.NotAfter.Unix()))
		}
	}

	privateKey := vals.Get("sslkey")
	certificate := vals.Get("sslcert")
	if privateKey == "" || certificate == "" {
		return trace.Errorf("certificate reloading enabled but sslcert or sslkey not present in connection string")
	}
	reloader := certreloader.New(certreloader.Config{
		KeyPairs: []servicecfg.KeyPairPath{
			{
				PrivateKey:  privateKey,
				Certificate: certificate,
			},
		},
		KeyPairsReloadInterval: reloadInterval,
	},
		slog.With(teleport.ComponentKey, teleport.Component(teleport.Component(teleport.ComponentAuth, "certreloader"), "name", name)), callback)
	if err := reloader.Run(ctx); err != nil {
		return trace.Wrap(err)
	}

	// When reload is enabled we need to check all the fallbacks as well as the main config because
	// of how sslmode is handled where the main config could be plaintext and falls back to a TLS
	// connection
	if connConfig.TLSConfig != nil {
		connConfig.TLSConfig.Certificates = nil
		connConfig.TLSConfig.GetClientCertificate = reloader.GetClientCertificate
	}

	for i, fallback := range connConfig.Fallbacks {
		if fallback.TLSConfig != nil {
			connConfig.Fallbacks[i].TLSConfig.Certificates = nil
			connConfig.Fallbacks[i].TLSConfig.GetClientCertificate = reloader.GetClientCertificate
		}
	}

	return nil
}
