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

package mongodb

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/x/mongo/driver/dns"
)

func setDefaultDNSResolver(t *testing.T, resolver *dns.Resolver) {
	t.Helper()
	// prevent parallel tests from running with the modified DNS resolver.
	t.Setenv("withDefaultDNSResolverDisallowsParallelTests", "1")
	original := dns.DefaultResolver
	t.Cleanup(func() {
		dns.DefaultResolver = original
	})
	dns.DefaultResolver = resolver
}
func TestNewEndpointsResolver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tests := []struct {
		desc          string
		uri           string
		dnsResolver   *dns.Resolver
		wantEndpoints []string
		wantErr       string
	}{
		{
			desc:          "simple host and port",
			uri:           "mongo1:27017",
			wantEndpoints: []string{"mongo1:27017"},
		},
		{
			desc:          "host without port",
			uri:           "mongo1",
			wantEndpoints: []string{"mongo1:27017"},
		},
		{
			desc:          "host with scheme",
			uri:           "mongodb://mongo1:27017",
			wantEndpoints: []string{"mongo1:27017"},
		},
		{
			desc:          "host without scheme",
			uri:           "mongo1:27017",
			wantEndpoints: []string{"mongo1:27017"},
		},
		{
			desc:          "replicaset",
			uri:           "mongodb://mongo1:27017,mongo2:27017/?replicaSet=rs0",
			wantEndpoints: []string{"mongo1:27017", "mongo2:27017"},
		},
		{
			desc:          "unresolvable SRV record error",
			uri:           "mongodb+srv://mongo1.invalid",
			wantEndpoints: []string{"mongo1:27017", "mongo2:27017"},
			wantErr:       "no such host",
		},
		{
			desc: "resolvable SRV record",
			uri:  "mongodb+srv://example.com",
			// fake the SRV record lookup for unit test.
			// I based the construction of these fake values by doing a real SRV
			// record lookup with the real default resolver
			dnsResolver: &dns.Resolver{
				LookupSRV: func(svc string, proto string, name string) (string, []*net.SRV, error) {
					target := fmt.Sprintf("_%v._%v.%v", svc, proto, name)
					records := []*net.SRV{
						{
							Target: "foo.com.",
							Port:   123,
						},
						{
							Target: "bar.com.",
							Port:   456,
						},
						{
							Target: "baz.com.",
							Port:   789,
						},
					}
					return target, records, nil
				},
				LookupTXT: func(string) ([]string, error) {
					// TXT record failures must not break seed list resolution
					return nil, trace.Errorf("some error")
				},
			},
			wantEndpoints: []string{"foo.com:123", "bar.com:456", "baz.com:789"},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			if test.dnsResolver != nil {
				setDefaultDNSResolver(t, test.dnsResolver)
			}
			resolver := newEndpointsResolver(test.uri)
			got, err := resolver.Resolve(ctx)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, test.wantEndpoints, got)
		})
	}
}
