// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fixtures

import (
	"time"
)

const (
	TLSCACertPEM = `-----BEGIN CERTIFICATE-----
MIIDKjCCAhKgAwIBAgIQJtJDJZZBkg/afM8d2ZJCTjANBgkqhkiG9w0BAQsFADBA
MRUwEwYDVQQKEwxUZWxlcG9ydCBPU1MxJzAlBgNVBAMTHnRlbGVwb3J0LmxvY2Fs
aG9zdC5sb2NhbGRvbWFpbjAeFw0xNzA1MDkxOTQwMzZaFw0yNzA1MDcxOTQwMzZa
MEAxFTATBgNVBAoTDFRlbGVwb3J0IE9TUzEnMCUGA1UEAxMedGVsZXBvcnQubG9j
YWxob3N0LmxvY2FsZG9tYWluMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKC
AQEAuKFLaf2iII/xDR+m2Yj6PnUEa+qzqwxsdLUjnunFZaAXG+hZm4Ml80SCiBgI
gTHQlJyLIkTtuRoH5aeMyz1ERUCtii4ZsTqDrjjUybxP4r+4HVX6m34s6hwEr8Fi
fts9pMp4iS3tQguRc28gPdDo/T6VrJTVYUfUUsNDRtIrlB5O9igqqLnuaY9eqGi4
PUx0G0wRYJpRywoj8G0IkpfQTiX+CAC7dt5ws7ZrnGqCNBLGi5bGsaMmptVbsSEp
1TenntF54V1iR49IV5JqDhm1S0HmkleoJzKdc+6sP/xNepz9PJzuF9d9NubTLWgB
sK28YItcmWHdHXD/ODxVaehRjwIDAQABoyAwHjAOBgNVHQ8BAf8EBAMCB4AwDAYD
VR0TAQH/BAIwADANBgkqhkiG9w0BAQsFAAOCAQEAAVU6sNBdj76saHwOxGSdnEqQ
o2tMuR3msSM4F6wFK2UkKepsD7CYIf/PzNSNUqA5JIEUVeMqGyiHuAbU4C655nT1
IyJX1D/+r73sSp5jbIpQm2xoQGZnj6g/Kltw8OSOAw+DsMF/PLVqoWJp07u6ew/m
NxWsJKcZ5k+q4eMxci9mKRHHqsquWKXzQlURMNFI+mGaFwrKM4dmzaR0BEc+ilSx
QqUvQ74smsLK+zhNikmgjlGC5ob9g8XkhVAkJMAh2rb9onDNiRl68iAgczP88mXu
vN/o98dypzsPxXmw6tkDqIRPUAUbh465rlY5sKMmRgXi2rUfl/QV5nbozUo/HQ==
-----END CERTIFICATE-----`
	TLSCAKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAuKFLaf2iII/xDR+m2Yj6PnUEa+qzqwxsdLUjnunFZaAXG+hZ
m4Ml80SCiBgIgTHQlJyLIkTtuRoH5aeMyz1ERUCtii4ZsTqDrjjUybxP4r+4HVX6
m34s6hwEr8Fifts9pMp4iS3tQguRc28gPdDo/T6VrJTVYUfUUsNDRtIrlB5O9igq
qLnuaY9eqGi4PUx0G0wRYJpRywoj8G0IkpfQTiX+CAC7dt5ws7ZrnGqCNBLGi5bG
saMmptVbsSEp1TenntF54V1iR49IV5JqDhm1S0HmkleoJzKdc+6sP/xNepz9PJzu
F9d9NubTLWgBsK28YItcmWHdHXD/ODxVaehRjwIDAQABAoIBABy4orWrShRMsA/9
k4QVpfAfXf+3tBlwxlJld1QaQ6XqgI3L2FyzyyyLxM6NBo2qhSsJKy+6j0yTOxVD
ukhHkJ5BUH3FbCPA2Yk5uAhl7ft1HZwaqvCTcUM99pCswbjAPFetU5DrfxQeHpNZ
fyd+ny/+E2SUhpkqhmIVlBqpSTQyOywbiEvZ6ZiFmncdHhXaCy3YZsylrKUGPzsJ
jfU2iOE167eTOIjPStsaoCPv9jLSyy2OvuNNudS+Y1qkFz8ZGvPp+HB+Iig+AlAE
7KMzNrIW7PlHTDgUly1cRCl3+84yE2mJ97+hHiEy//HIwVDUpI529i2hMYM/u4qz
Wso/2tkCgYEA2FdE4bmCrZiA9eS8qobwGLE1+MJME4YwfJkynZUHHX93xORPQ66e
WYpN7/xbMvBDa8LZZYVTNVtZ/SkEUaTb5NQW2zXKoIutk1PFBb8NbA0m8Ss/mOJA
d5nUYGr987O9fRh1yP9TksBshHB/5A8U2UG8MFFCNvJTZDPRkuSlMiUCgYEA2nnb
hAJrhY7PaF6jdfimGvvponkUiEbWLppg7/SjgPg+QgqIwuLybryXyOAp+TEnNzgU
ujAjhNtIiyB/B13TDxOgUgWUWPbPvUAWGEvwI9h+RLie1umGHd48G1NR76fwqSf1
y7z3YRnq8vCdz8ywB3o5GO6SH6QkMJBIxfIMlKMCgYA55akOi7oYQT8KD4waSwCI
ayyZhU4cz4W8Yrd0CsUbtNhVvhAked/w8J2JA01Y5Yn1lfDeRX8OQYNkyAxa2Tbs
F4KCafPvYVIzonCQ6B9sclygoEVl4e8E0wtOPnP2O30TtG8ZOpOgK5UfIIhpfUvE
FN6LQ8PntpRwtZl5qW04bQKBgGnHhFxHG64fthZPdA9jY3E/NSCgRSuyOHN59aNY
rG1+RA6PsSXC4iRxlYAB4PCxNs6KjaaUNi5WSaprAnYbnFv5Ya802l20qmJ0C/6Z
jdydLo2xYd6mVHRTrICCd/J0OpZ8LYsGpDPUa6hSjeYVscj9CXYj1IYTYB5PTZzh
k+vHAoGBAJyA+RtBF5m64/TqhZFcesTtnpWaRhQ50xXnNVF3W1eKGPtdTDKOaENA
LJxgC1GdoEz2ilXW802H9QrdKf9GPqxwi2TVzfO6pzWkdZcmbItu+QCCFz+co+r8
+ki49FmlfbR5YVPN+8X40aLQB4xDkCHwRwTkrigzWQhIOv8NAhDA
-----END RSA PRIVATE KEY-----`
)

var (
	// TLSCACertNotBefore is the "Not before" date of TLSCACertPEM.
	TLSCACertNotBefore = time.Date(2017, time.May, 9, 19, 40, 36, 0, time.UTC)
	// TLSCACertNotAfter is the "Not after" date of TLSCACertPEM.
	TLSCACertNotAfter = time.Date(2027, time.May, 7, 19, 40, 36, 0, time.UTC)
)
