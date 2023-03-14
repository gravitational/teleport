// Copyright 2021 Gravitational, Inc
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

package utils

import (
	"bytes"
	"encoding/base32"
	"image/png"
	"net/url"

	"github.com/gravitational/trace"
	"github.com/pquerna/otp"
)

// GenerateQRCode takes in a OTP Key URL and returns a PNG-encoded QR code.
func GenerateQRCode(u string) ([]byte, error) {
	otpKey, err := otp.NewKeyFromURL(u)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otpImage, err := otpKey.Image(450, 450)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var otpQR bytes.Buffer
	err = png.Encode(&otpQR, otpImage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return otpQR.Bytes(), nil
}

// GenerateOTPURL returns a OTP Key URL that can be used to construct a HOTP or TOTP key. For more
// details see: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
// Example: otpauth://totp/foo:bar@baz.com?secret=qux
func GenerateOTPURL(typ string, label string, parameters map[string][]byte) string {
	var u url.URL

	u.Scheme = "otpauth"
	u.Host = typ
	u.Path = label

	params := make(url.Values)
	for k, v := range parameters {
		if k == "secret" {
			v = []byte(base32.StdEncoding.EncodeToString(v))
		}
		params.Add(k, string(v))
	}
	u.RawQuery = params.Encode()

	return u.String()
}
