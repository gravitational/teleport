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

package hotp

import (
	"github.com/pquerna/otp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"encoding/base32"
	"testing"
)

type tc struct {
	Counter uint64
	TOTP    string
	Mode    otp.Algorithm
	Secret  string
}

var (
	secSha1 = base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))

	rfcMatrixTCs = []tc{
		tc{0, "755224", otp.AlgorithmSHA1, secSha1},
		tc{1, "287082", otp.AlgorithmSHA1, secSha1},
		tc{2, "359152", otp.AlgorithmSHA1, secSha1},
		tc{3, "969429", otp.AlgorithmSHA1, secSha1},
		tc{4, "338314", otp.AlgorithmSHA1, secSha1},
		tc{5, "254676", otp.AlgorithmSHA1, secSha1},
		tc{6, "287922", otp.AlgorithmSHA1, secSha1},
		tc{7, "162583", otp.AlgorithmSHA1, secSha1},
		tc{8, "399871", otp.AlgorithmSHA1, secSha1},
		tc{9, "520489", otp.AlgorithmSHA1, secSha1},
	}
)

// Test values from http://tools.ietf.org/html/rfc4226#appendix-D
func TestValidateRFCMatrix(t *testing.T) {

	for _, tx := range rfcMatrixTCs {
		valid, err := ValidateCustom(tx.TOTP, tx.Counter, tx.Secret,
			ValidateOpts{
				Digits:    otp.DigitsSix,
				Algorithm: tx.Mode,
			})
		require.NoError(t, err,
			"unexpected error totp=%s mode=%v counter=%v", tx.TOTP, tx.Mode, tx.Counter)
		require.True(t, valid,
			"unexpected totp failure totp=%s mode=%v counter=%v", tx.TOTP, tx.Mode, tx.Counter)
	}
}

func TestGenerateRFCMatrix(t *testing.T) {
	for _, tx := range rfcMatrixTCs {
		passcode, err := GenerateCodeCustom(tx.Secret, tx.Counter,
			ValidateOpts{
				Digits:    otp.DigitsSix,
				Algorithm: tx.Mode,
			})
		assert.Nil(t, err)
		assert.Equal(t, tx.TOTP, passcode)
	}
}

func TestValidateInvalid(t *testing.T) {
	secSha1 := base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))

	valid, err := ValidateCustom("foo", 11, secSha1,
		ValidateOpts{
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
	require.Equal(t, otp.ErrValidateInputInvalidLength, err, "Expected Invalid length error.")
	require.Equal(t, false, valid, "Valid should be false when we have an error.")

	valid, err = ValidateCustom("foo", 11, secSha1,
		ValidateOpts{
			Digits:    otp.DigitsEight,
			Algorithm: otp.AlgorithmSHA1,
		})
	require.Equal(t, otp.ErrValidateInputInvalidLength, err, "Expected Invalid length error.")
	require.Equal(t, false, valid, "Valid should be false when we have an error.")

	valid, err = ValidateCustom("000000", 11, secSha1,
		ValidateOpts{
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
	require.NoError(t, err, "Expected no error.")
	require.Equal(t, false, valid, "Valid should be false.")

	valid = Validate("000000", 11, secSha1)
	require.Equal(t, false, valid, "Valid should be false.")
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

	k, err = Generate(GenerateOpts{
		Issuer:      "",
		AccountName: "alice@example.com",
	})
	require.Equal(t, otp.ErrGenerateMissingIssuer, err, "generate missing issuer")
	require.Nil(t, k, "key should be nil on error.")

	k, err = Generate(GenerateOpts{
		Issuer:      "Foobar, Inc",
		AccountName: "",
	})
	require.Equal(t, otp.ErrGenerateMissingAccountName, err, "generate missing account name.")
	require.Nil(t, k, "key should be nil on error.")
}
