/*
Copyright 2022 Gravitational, Inc.

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

package web

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateDatabaseRequestParameters(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		desc      string
		req       createDatabaseRequest
		errAssert require.ErrorAssertionFunc
	}{
		{
			desc: "valid",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "protocol",
				URI:      "uri",
			},
			errAssert: require.NoError,
		},
		{
			desc: "invalid missing name",
			req: createDatabaseRequest{
				Name:     "",
				Protocol: "protocol",
				URI:      "uri",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing protocol",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "",
				URI:      "uri",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid missing uri",
			req: createDatabaseRequest{
				Name:     "name",
				Protocol: "protocol",
				URI:      "",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssert(t, test.req.checkAndSetDefaults())
		})
	}
}

var fakeValidTLSCert = `-----BEGIN CERTIFICATE-----
MIIDyzCCArOgAwIBAgIQD3MiJ2Au8PicJpCNFbvcETANBgkqhkiG9w0BAQsFADBe
MRQwEgYDVQQKEwtleGFtcGxlLmNvbTEUMBIGA1UEAxMLZXhhbXBsZS5jb20xMDAu
BgNVBAUTJzIwNTIxNzE3NzMzMTIxNzQ2ODMyNjA5NjAxODEwODc0NTAzMjg1ODAe
Fw0yMTAyMTcyMDI3MjFaFw0yMTAyMTgwODI4MjFaMIGCMRUwEwYDVQQHEwxhY2Nl
c3MtYWRtaW4xCTAHBgNVBAkTADEYMBYGA1UEEQwPeyJsb2dpbnMiOm51bGx9MRUw
EwYDVQQKEwxhY2Nlc3MtYWRtaW4xFTATBgNVBAMTDGFjY2Vzcy1hZG1pbjEWMBQG
BSvODwEHEwtleGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoC
ggEBAM5FFaCeK59lwIthyXgSCMZbHTDxsy66Cbm/XhwFbKQLngyS0oKkHbh06INN
UfTAAEaFlMG0CzdAyGyRSu9FK8BE127kRHBs6hb1pTgy2f6TFkFo/h4WTWW4GQSi
O8Al7A2tuRjc3mAnk71q+kvpQYS7tnkhmFCYE8jKxMtlYG39x4kQ6btll7P9zI6X
Zv5RRrlzqADuwZpEcLYVi0TjITqPbx3rDZT4l+EmslhaoG+xE5Vu+GYXLlvwB9E/
amfN1Z9Kps4Ob6Jxxse9kjeMir9mwiNkBWVyhH/LETDA9Xa6sTQ2e75MYM7yXJLY
OmBKV4g176Qf1T1ye7a/Ggn4t2UCAwEAAaNgMF4wDgYDVR0PAQH/BAQDAgWgMB0G
A1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB8GA1Ud
IwQYMBaAFJWqMooE05nf263F341pOO+mPMSqMA0GCSqGSIb3DQEBCwUAA4IBAQCK
s0yPzkSuCY/LFeHJoJeNJ1SR+EKbk4zoAnD0nbbIsd2quyYIiojshlfehhuZE+8P
bzpUNG2aYKq+8lb0NO+OdZW7kBEDWq7ZwC8OG8oMDrX385fLcicm7GfbGCmZ6286
m1gfG9yqEte7pxv3yWM+7X2bzEjCBds4feahuKPNxOAOSfLUZiTpmOVlRzrpRIhu
2XxiuH+E8n4AP8jf/9bGvKd8PyHohtHVf8HWuKLZxWznQhoKkcfmUmlz5q8ci4Bq
WQdM2NXAMABGAofGrVklPIiraUoHzr0Xxpia4vQwRewYXv8bCPHW+8g8vGBGvoG2
gtLit9DL5DR5ac/CRGJt
-----END CERTIFICATE-----`

func TestUpdateDatabaseRequestParameters(t *testing.T) {

	for _, test := range []struct {
		desc      string
		req       updateDatabaseRequest
		errAssert require.ErrorAssertionFunc
	}{
		{
			desc: "valid",
			req: updateDatabaseRequest{
				CACert: fakeValidTLSCert,
			},
			errAssert: require.NoError,
		},
		{
			desc: "invalid missing ca_cert",
			req: updateDatabaseRequest{
				CACert: "",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
		{
			desc: "invalid ca_cert format",
			req: updateDatabaseRequest{
				CACert: "ca_cert",
			},
			errAssert: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err), "expected a bad parameter error, got", err)
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssert(t, test.req.checkAndSetDefaults())
		})
	}
}
