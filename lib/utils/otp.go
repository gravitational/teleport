package utils

import (
	"encoding/base32"
	"net/url"
)

// GenerateOTPURL returns a OTP Key URL that can be used to construct a HOTP or TOTP key. For more
// details see: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
// Example: otpauth://totp/foo:bar@baz.com?secret=qux
func GenerateOTPURL(typ string, label string, parameters map[string][]byte) string {
	var u url.URL

	u.Scheme = "otpauth"
	u.Host = typ
	u.Path = label

	var params url.Values = make(url.Values)
	for k, v := range parameters {
		if k == "secret" {
			v = []byte(base32.StdEncoding.EncodeToString(v))
		}
		params.Add(k, string(v))
	}
	u.RawQuery = params.Encode()

	return u.String()
}
