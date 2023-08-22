/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package webauthncli

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func unverifiedBase64Bytes(str string) []byte {
	bytes, _ := base64.StdEncoding.DecodeString(str)
	return bytes
}

func FuzzParseU2FRegistrationResponse(f *testing.F) {
	f.Add(unverifiedBase64Bytes("BQR+GkzX1lnNopfxpz1baMSaU1wlqZaJ7tGrOJ14p" +
		"QucBTZR4sKwiJZTuponQvXwJuj3zdanzMH1Os7pjFy4IbEegHIXW/sZVdiZUsjdUQH6/WD0" +
		"4rllLPEYiiocu/fS1zmntWNBAwI1DOgGJ4FSDsAIidZekwAapqsln+RaNiUgvC4WY0qSYGl" +
		"3uDz2O6jbaBCjTcLzifcjyaQb3KGLs3EEPN1eNeJcjACVpyWUMZDSOlFkFaE4q0QMJqCCS3" +
		"c3ng/cMIIBazCCARCgAwIBAgIBATAKBggqhkjOPQQDAjASMRAwDgYDVQQKEwdUZXN0IENBM" +
		"B4XDTIzMDgxNjE4MjAwNloXDTIzMDgxNjE5MjAwNlowEjEQMA4GA1UEChMHVGVzdCBDQTBZ" +
		"MBMGByqGSM49AgEGCCqGSM49AwEHA0IABH4aTNfWWc2il/GnPVtoxJpTXCWplonu0as4nXi" +
		"lC5wFNlHiwrCIllO6midC9fAm6PfN1qfMwfU6zumMXLghsR6jVzBVMA4GA1UdDwEB/wQEAw" +
		"ICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRF/" +
		"vV/LdwWaAA1LA3uAYj9ErbjVzAKBggqhkjOPQQDAgNJADBGAiEAixVchjFZ+oEhTXJYCUtx" +
		"xi/z4PooqF/tlNGKPUHPD6QCIQCqo129HBg5QaUjXc7dHxGVc3joct+CTSIwtyUKSN6twTB" +
		"GAiEApJfP1bm0/sZTUZ8XeN86WdHVb4+Qz3lwB0d1GxkYM7YCIQCJyXkyu4Y7bm0YPP+XB8" +
		"3IO2WCmJKNsCT8sZuRRs/ryw=="))
	f.Add(unverifiedBase64Bytes("BQSEpSKEdxODGvlDbmWKkhqTzCriCEb72v5+dh1mf" +
		"rZwPxa2DihjLO4LrrN79bz/IYT4AtlNlwP3mDDmv1dhl5XpgH5OJ92XUa+lHeR/ScWXrlld" +
		"5saUtmuA9Osg3UFK2wActU2Yq0yT8pEzECZba/npHDmSHFs25i0FWiy7ZSSE0hyi2mACyXm" +
		"yLyRyEg6mH84aVMvW9M0QjFMDmjaZpqcFbXVkf7luOrvLhzo2kUd4fgAZ5bsVlb6Ggfl7Kb" +
		"0q3MPVMIIBajCCARCgAwIBAgIBATAKBggqhkjOPQQDAjASMRAwDgYDVQQKEwdUZXN0IENBM" +
		"B4XDTIzMDgxNjE4MjIyMFoXDTIzMDgxNjE5MjIyMFowEjEQMA4GA1UEChMHVGVzdCBDQTBZ" +
		"MBMGByqGSM49AgEGCCqGSM49AwEHA0IABISlIoR3E4Ma+UNuZYqSGpPMKuIIRvva/n52HWZ" +
		"+tnA/FrYOKGMs7guus3v1vP8hhPgC2U2XA/eYMOa/V2GXlemjVzBVMA4GA1UdDwEB/wQEAw" +
		"ICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRHM" +
		"KsFLCtx6PUVkpDw8DdKQf9C0zAKBggqhkjOPQQDAgNIADBFAiByA6ISaK+iwQ7TC40IPMXm" +
		"mHzIf32b0YZwsHTUNf5jDgIhAPDBB5n3wR4d3F+R2PkvbwneqwcwkrrEzpBEXwwsEhpOMEQ" +
		"CIFAYEWOJZevn6IxtTBg5w/krrHA9z0pzAHRs13KOPEHEAiArbTczB8nS3HIeCJqUt8wclg" +
		"TVPnbu99FYtP5FueW8Hg=="))

	f.Fuzz(func(t *testing.T, b []byte) {
		require.NotPanics(t, func() {
			_, _ = parseU2FRegistrationResponse(b)
		})
	})
}
