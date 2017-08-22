/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package totp

import (
	"github.com/pquerna/otp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/base32"
	"testing"
	"time"
)

type tc struct {
	TS     int64
	TOTP   string
	Mode   otp.Algorithm
	Secret string
}

var (
	secSha1   = base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	secSha256 = base32.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	secSha512 = base32.StdEncoding.EncodeToString([]byte("1234567890123456789012345678901234567890123456789012345678901234"))

	rfcMatrixTCs = []tc{
		tc{59, "94287082", otp.AlgorithmSHA1, secSha1},
		tc{59, "46119246", otp.AlgorithmSHA256, secSha256},
		tc{59, "90693936", otp.AlgorithmSHA512, secSha512},
		tc{1111111109, "07081804", otp.AlgorithmSHA1, secSha1},
		tc{1111111109, "68084774", otp.AlgorithmSHA256, secSha256},
		tc{1111111109, "25091201", otp.AlgorithmSHA512, secSha512},
		tc{1111111111, "14050471", otp.AlgorithmSHA1, secSha1},
		tc{1111111111, "67062674", otp.AlgorithmSHA256, secSha256},
		tc{1111111111, "99943326", otp.AlgorithmSHA512, secSha512},
		tc{1234567890, "89005924", otp.AlgorithmSHA1, secSha1},
		tc{1234567890, "91819424", otp.AlgorithmSHA256, secSha256},
		tc{1234567890, "93441116", otp.AlgorithmSHA512, secSha512},
		tc{2000000000, "69279037", otp.AlgorithmSHA1, secSha1},
		tc{2000000000, "90698825", otp.AlgorithmSHA256, secSha256},
		tc{2000000000, "38618901", otp.AlgorithmSHA512, secSha512},
		tc{20000000000, "65353130", otp.AlgorithmSHA1, secSha1},
		tc{20000000000, "77737706", otp.AlgorithmSHA256, secSha256},
		tc{20000000000, "47863826", otp.AlgorithmSHA512, secSha512},
	}
)

//
// Test vectors from http://tools.ietf.org/html/rfc6238#appendix-B
// NOTE -- the test vectors are documented as having the SAME
// secret -- this is WRONG -- they have a variable secret
// depending upon the hmac algorithm:
// 		http://www.rfc-editor.org/errata_search.php?rfc=6238
// this only took a few hours of head/desk interaction to figure out.
//
func TestValidateRFCMatrix(t *testing.T) {
	for _, tx := range rfcMatrixTCs {
		valid, err := ValidateCustom(tx.TOTP, tx.Secret, time.Unix(tx.TS, 0).UTC(),
			ValidateOpts{
				Digits:    otp.DigitsEight,
				Algorithm: tx.Mode,
			})
		require.NoError(t, err,
			"unexpected error totp=%s mode=%v ts=%v", tx.TOTP, tx.Mode, tx.TS)
		require.True(t, valid,
			"unexpected totp failure totp=%s mode=%v ts=%v", tx.TOTP, tx.Mode, tx.TS)
	}
}

func TestGenerateRFCTCs(t *testing.T) {
	for _, tx := range rfcMatrixTCs {
		passcode, err := GenerateCodeCustom(tx.Secret, time.Unix(tx.TS, 0).UTC(),
			ValidateOpts{
				Digits:    otp.DigitsEight,
				Algorithm: tx.Mode,
			})
		assert.Nil(t, err)
		assert.Equal(t, tx.TOTP, passcode)
	}
}

func TestValidateSkew(t *testing.T) {
	secSha1 := base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))

	tests := []tc{
		tc{29, "94287082", otp.AlgorithmSHA1, secSha1},
		tc{59, "94287082", otp.AlgorithmSHA1, secSha1},
		tc{61, "94287082", otp.AlgorithmSHA1, secSha1},
	}

	for _, tx := range tests {
		valid, err := ValidateCustom(tx.TOTP, tx.Secret, time.Unix(tx.TS, 0).UTC(),
			ValidateOpts{
				Digits:    otp.DigitsEight,
				Algorithm: tx.Mode,
				Skew:      1,
			})
		require.NoError(t, err,
			"unexpected error totp=%s mode=%v ts=%v", tx.TOTP, tx.Mode, tx.TS)
		require.True(t, valid,
			"unexpected totp failure totp=%s mode=%v ts=%v", tx.TOTP, tx.Mode, tx.TS)
	}
}

func TestGenerate(t *testing.T) {
	k, err := Generate(GenerateOpts{
		Issuer:      "SnakeOil",
		AccountName: "alice@example.com",
	})
	require.NoError(t, err, "generate basic TOTP")
	require.Equal(t, "SnakeOil", k.Issuer(), "Extracting Issuer")
	require.Equal(t, "alice@example.com", k.AccountName(), "Extracting Account Name")
	require.Equal(t, 16, len(k.Secret()), "Secret is 16 bytes long as base32.")

	k, err = Generate(GenerateOpts{
		Issuer:      "SnakeOil",
		AccountName: "alice@example.com",
		SecretSize:  20,
	})
	require.NoError(t, err, "generate larger TOTP")
	require.Equal(t, 32, len(k.Secret()), "Secret is 32 bytes long as base32.")
}
