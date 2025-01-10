/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
package tpm

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tpmEKCertPEM is the real RSA 2048 EK certificate. This was captured from
// Noah's Infineon SLB9665 TPM.
const tpmEKCertPEM = `-----BEGIN CERTIFICATE-----
MIIElTCCA32gAwIBAgIEXs1fjjANBgkqhkiG9w0BAQsFADCBgzELMAkGA1UEBhMC
REUxITAfBgNVBAoMGEluZmluZW9uIFRlY2hub2xvZ2llcyBBRzEaMBgGA1UECwwR
T1BUSUdBKFRNKSBUUE0yLjAxNTAzBgNVBAMMLEluZmluZW9uIE9QVElHQShUTSkg
UlNBIE1hbnVmYWN0dXJpbmcgQ0EgMDM2MB4XDTIzMDExNzA4MDY0MVoXDTM3MDgy
MDIzNTk1OVowADCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAJ0qMMu+
SRyCrhKcEQvKXk+Md2rdZC317Nqmhjf7rXJ527DX051XMTKfy+SXfalkqdT8IQkd
aUPYC/m8XWz7/J/9781dVt7rOw1CJsEk9DFoaInQmL2E5dUDgsA8Em942o2r1x7K
NdigHrLRQetn/CJkODYeBnHmmQUpU9syZ86Dhxl5tK1Sq2ddCm5Z/RCy+LIRBrpl
qstrTsY3Wyj0aqt/Opikq3geSkW+viG9ipk/D5J3i/qbdHQHSWZqD6ImixTmqIZf
I3u9QCVftoeWuJxrXgUPHdyyO6lMXDUgW918912Ihr6ZRBY6jUZT2Y8II+T1IRGT
/ymr26W7Mf7qfYkCAwEAAaOCAZEwggGNMFsGCCsGAQUFBwEBBE8wTTBLBggrBgEF
BQcwAoY/aHR0cDovL3BraS5pbmZpbmVvbi5jb20vT3B0aWdhUnNhTWZyQ0EwMzYv
T3B0aWdhUnNhTWZyQ0EwMzYuY3J0MA4GA1UdDwEB/wQEAwIAIDBRBgNVHREBAf8E
RzBFpEMwQTEWMBQGBWeBBQIBDAtpZDo0OTQ2NTgwMDETMBEGBWeBBQICDAhTTEIg
OTY2NTESMBAGBWeBBQIDDAdpZDowNTNmMAwGA1UdEwEB/wQCMAAwUAYDVR0fBEkw
RzBFoEOgQYY/aHR0cDovL3BraS5pbmZpbmVvbi5jb20vT3B0aWdhUnNhTWZyQ0Ew
MzYvT3B0aWdhUnNhTWZyQ0EwMzYuY3JsMBUGA1UdIAQOMAwwCgYIKoIUAEQBFAEw
HwYDVR0jBBgwFoAUfLS3jmiGFL5EIcWFjxW5bV6rUe4wEAYDVR0lBAkwBwYFZ4EF
CAEwIQYDVR0JBBowGDAWBgVngQUCEDENMAsMAzIuMAIBAAIBdDANBgkqhkiG9w0B
AQsFAAOCAQEACSSM+6o4INqV7mJ+aD5kPH6BkbEPhJBsYRA6vka+911Th7JfGZA7
4C1ig4EjD1qUaRvkwNoDbGr3MRiNPHan3PLJkBy+WSERWglnBlXooJnkncWsNGwm
lzCTAYPOKZSTLiZiijvzW1XO+VqaTCMTkTpegO3MnE6xXZhXSyXQs8ro7qY6cTBd
whcEtufTT4khxMhjRTUBocqlLlN8PifG6xL2GD6xAW/PplL0uQFLUgnY0U4WQ/oP
nK/NX7N02p23JzlhDgcOdRrF3hYi8huKuoe3YfVJCTJLfSwxHytgXqJdpiRSHvut
Upw0EsfcY7cSbnlMWx2n4c7ptVvqiLXTkg==
-----END CERTIFICATE-----`

func TestStripSANExtensionsOIDs(t *testing.T) {
	b, _ := pem.Decode([]byte(tpmEKCertPEM))
	require.NotNil(t, b)
	c, err := x509.ParseCertificate(b.Bytes)
	require.NoError(t, err)

	assert.Len(t, c.UnhandledCriticalExtensions, 1)
	StripSANExtensionOIDs(c)
	assert.Empty(t, c.UnhandledCriticalExtensions)
}
