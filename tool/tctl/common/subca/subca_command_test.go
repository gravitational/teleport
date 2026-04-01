// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package subca_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/tool/tctl/common/subca"
)

func TestCommand_PubKeyHash(t *testing.T) {
	t.Parallel()

	const certPEM = `-----BEGIN CERTIFICATE-----
MIIERzCCAq+gAwIBAgIRAMZcaomNE+VygiNyxoSwzr8wDQYJKoZIhvcNAQELBQAw
fTEeMBwGA1UEChMVbWtjZXJ0IGRldmVsb3BtZW50IENBMSkwJwYDVQQLDCBhbGFu
QHphcnF1b24yLmxvY2FsIChBbGFuIFBhcnJhKTEwMC4GA1UEAwwnbWtjZXJ0IGFs
YW5AemFycXVvbjIubG9jYWwgKEFsYW4gUGFycmEpMB4XDTI2MDMzMDE0NTI0OVoX
DTI4MDYzMDE0NTI0OVowSTEnMCUGA1UEChMebWtjZXJ0IGRldmVsb3BtZW50IGNl
cnRpZmljYXRlMR4wHAYDVQQLDBVhbGFuQE1hYyAoQWxhbiBQYXJyYSkwggEiMA0G
CSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDb43vlSk68rJMK/kKjGEP06y81A4kt
e+sfwsGXbevwk0565FfXsN3oraUkb5QG0/Cv6pZotBMOx8g8Gbkb0Od4OoESD9l1
SQF1F+9DNyrGQscLg9cz+qVaG9q0DIwGMKNmchZwZES0PGeA6l6CAgsR06NoTht3
Cv8id6myHlOg+pKoil5iQ9kydxrU9qku4dRlYggbMynn9KaEm0i5g7ipIsnvnRkL
WOJup9qLPYdQIwQfKtQiJ6SG95pypnkxku4qbPsWKqReYH694HwWBo5XKERbFEX2
5lQNGpwWRLFxquZiheiu7C/jxxzxLnxnScGcERpI6om3plK1OgbRPx9RAgMBAAGj
djB0MA4GA1UdDwEB/wQEAwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATAfBgNVHSME
GDAWgBRIeRs75N2PBLqa0s7ILgxvFho9PTAsBgNVHREEJTAjgglsb2NhbGhvc3SH
BH8AAAGHEAAAAAAAAAAAAAAAAAAAAAEwDQYJKoZIhvcNAQELBQADggGBAA52BvfF
Xs084HMVyRu09FYwu9fiHfD3Whyq7khPhBhF9/r+/VBEjie5oi4Vk9z/OVWPSp37
XN+UeNvAHQh07ntAW+7m/Gfrt4lX7kK46+Gjgzu8YPBNhAkYEhJz8ViJshSSfRnu
itwRn/9J7qVzGud6Hhn+Zc+CBrIe467jT+iUoA1iiZDUjZHqo1O14SwomuB2/CzC
t4C4ZQSxU6yOEcvJP6C0hwSZYKht5+WGwLy95cvX9kk+RrqEysLatbUfr1xrE35y
2VEVbb8ppWMrd6njbJjGXBO1bb1kNUgj0GYI+0oJeu5W4AXBYS97tAntNWFcfeFS
O/sfRlbBpQNYxA045fO/Dc3qKhZAVfLLWL/mK3XSRS8gz0iBESlDXDVqChKeOzZV
Fv4gVOCfPBtGRcyntw5YMjRBr1M5E7lR7f0EiEf98XK+7qCyrgBa63/F1YConCCK
OCILEpSS/B19NPlXOYipYx7lPCoRHCsDQk/RuQ0doilw+Gx1f/lMMH3Z/g==
-----END CERTIFICATE-----`
	// Pre-calculated pub key hash of certPEM.
	const want = "0fa1e6306b74b68aca58636482dc3cb5ff64ba17dbff6d808cf901169745e6be\n"

	tests := []struct {
		name           string
		prepareCommand func(t *testing.T, stdin *bytes.Buffer) []string
	}{
		{
			name: "--cert=file",
			prepareCommand: func(t *testing.T, stdin *bytes.Buffer) []string {
				tmp := t.TempDir()
				name := tmp + "/cert.pem"
				require.NoError(t,
					os.WriteFile(name, []byte(certPEM), 0644),
					"Write test certificate",
				)
				// "tctl auth pub-key-hash --cert=cert.pem"
				return []string{"auth", "pub-key-hash", "--cert", name}
			},
		},
		{
			name: "--cert='-' (stdin)",
			prepareCommand: func(t *testing.T, stdin *bytes.Buffer) []string {
				stdin.WriteString(certPEM)
				// "tctl auth pub-key-hash --cert='-'"
				return []string{"auth", "pub-key-hash", "--cert=-"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			env := newCommand(t)

			// Prepare/parse flags.
			flags := test.prepareCommand(t, env.Stdin)
			selectedCommand, err := env.App.Parse(flags)
			require.NoError(t, err, "app.Parse()")

			// Run.
			match, err := env.Cmd.TryRun(t.Context(), selectedCommand, nil)
			require.True(t, match, "TryRun() returned match=false")
			require.NoError(t, err, "TryRun() errored")

			// Verify output.
			got := env.Stdout.String()
			assert.Equal(t, want, got, "pub-key-hash output mismatch")
		})
	}
}

type commandEnv struct {
	App                   *kingpin.Application
	Cmd                   *subca.Command
	Stdin, Stdout, Stderr *bytes.Buffer
}

func newCommand(_ *testing.T) *commandEnv {
	app := kingpin.New("tctl", "")
	authCmd := app.Command("auth", "")

	stdin := &bytes.Buffer{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	subCACmd := &subca.Command{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	subCACmd.Initialize(authCmd, nil, nil)

	return &commandEnv{
		App:    app,
		Cmd:    subCACmd,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
}
