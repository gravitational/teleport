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

package services

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func FuzzParseRefs(f *testing.F) {
	// seeds from unit test examples
	f.Add("lock")
	f.Add("integration")
	f.Add("integration/00124f1e-d70e-413e-9b20-9b2d4c97e10c")
	f.Add("integration/unknown")
	f.Add("integration/myawsint")
	f.Add("app")
	f.Add("app/appB")
	f.Add("db_server")
	f.Add("db_server/example")
	f.Add("db")
	f.Add("db/example")
	f.Add("db_service")
	f.Add("db_service/7af76d49-b747-4bc1-b43d-c6dd457c229e")
	f.Add("db_service/unknown")
	// other seeds
	f.Add("foo,bar")
	f.Add("foo\\,bar/foobar")

	f.Fuzz(func(t *testing.T, refs string) {
		require.NotPanics(t, func() {
			ParseRefs(refs)
		})
	})
}

func FuzzParserEvalBoolPredicate(f *testing.F) {
	// seeds from unit tests
	f.Add("name == \"4a6t1q1zcsq97q\"")
	f.Add("labels.env == \"test\"")
	f.Add("contains(reviewer.roles,\"dev\")")
	f.Add("!contains(reviewer.traits[\"teams\"],\"staging-admin\")")
	f.Add("equals(request.reason,review.reason)")
	f.Add("contains(reviewer.roles, \"admin\")")
	f.Add("equals(fully.fake.path,\"should-fail\")")
	f.Add("fakefunc(reviewer.roles,\"some-role\")")
	f.Add("equals(\"too\",\"many\",\"params\")")
	f.Add("contains(\"missing-param\")")
	f.Add("&& missing-left")
	f.Add("labels.env.toomanyfield")
	f.Add("exists(labels.undefined)")
	f.Add("name.toomanyfield")
	f.Add("!name")
	f.Add("name ==")
	f.Add("equals(labels[\"env\"], \"wrong-value\")")
	f.Add("name ||")
	f.Add("&&")
	f.Add("||")
	f.Add("|")
	f.Add("&")
	f.Add("!")
	f.Add(".")
	f.Add("!exists(labels.env)")
	f.Add("name &&")
	f.Add("name &")
	f.Add("name |")
	f.Add("search(\"mac\", \"not-found\")")
	f.Add("hasPrefix(name, \"x\")")
	f.Add("search(\"mac\")")
	f.Add("equals()")
	f.Add("exists()")
	f.Add("search(1,2)")
	f.Add("\"just-string\"")
	f.Add("hasPrefix(1, 2)")
	f.Add("hasPrefix(name, \"too\", \"many\")")
	f.Add("hasPrefix(name, 1)")
	f.Add("search()")
	f.Add("resource.metadata.labels[\"env\"] == \"prod\"")
	f.Add("(exists(labels.env) || exists(labels.os)) && labels.os != \"mac\"")
	f.Add("search(\"does\", \"not\", \"exist\") || resource.spec.addr == \"_\" || labels.version == \"v8\"")

	f.Fuzz(func(t *testing.T, expr string) {
		resource, err := types.NewServerWithLabels("test-name", types.KindNode, types.ServerSpecV2{
			Hostname: "test-hostname",
			Addr:     "test-addr",
			CmdLabels: map[string]types.CommandLabelV2{
				"version": {
					Result: "v8",
				},
			},
		}, map[string]string{
			"env": "prod",
			"os":  "mac",
		})
		require.NoError(t, err)

		require.NotPanics(t, func() {
			parser, err := NewResourceExpression(expr)
			if err != nil {
				return
			}

			parser.Evaluate(resource)
		})
	})
}

func FuzzValidateApp(f *testing.F) {
	f.Add("web.example.com:443", "app.example.com")         // valid: different addresses
	f.Add("", "app.example.com")                            // valid: empty proxy address
	f.Add("proxy.example.com:443", "")                      // valid: empty app address
	f.Add("example.com", "app.example.com")                 // valid: proxy without port
	f.Add("web.example.com:443", "web.example.com")         // conflict: same as proxy
	f.Add("web.example.com:443", "web.example.com.")        // conflict: trailing dot
	f.Add("web.example.com:443", "web.example.com..")       // conflict: multiple trailing dots
	f.Add("web.example.com:443", "WeB.ExAmPle.CoM")         // conflict: case insensitive
	f.Add("web.example.com:443,other.com:443", "other.com") // conflict: matches second proxy
	f.Add("xn--mnchen-3ya.de:443", "münchen.de")            // conflict: IDN
	f.Add("münchen.de:443", "MünchEn.de")                   // conflict: IDN case insensitive
	f.Add("example.com:443,example.com:80", "example.com")  // conflict: multiple proxy ports

	f.Fuzz(func(t *testing.T, proxyPublicAddrs string, appPublicAddr string) {
		// NewAppV3 and ValidateApp should never panic regardless of input.
		require.NotPanics(t, func() {
			// Create app with a fuzzy public address.
			app, err := types.NewAppV3(types.Metadata{Name: "fuzz-app"}, types.AppSpecV3{
				URI:        "http://localhost:8080",
				PublicAddr: appPublicAddr,
			})
			if err != nil {
				// Fuzzing may produce invalid values that fail to create an App.
				// If so, skip this iteration since the test cannot continue.
				t.Skip("skipping invalid app spec")
			}

			proxyAddrList := strings.Split(proxyPublicAddrs, ",")
			mockProxyGetter := &mockProxyGetter{addrs: proxyAddrList}

			// Validate the app against the mock proxy addresses.
			_ = ValidateApp(app, mockProxyGetter)
		})
	})
}
