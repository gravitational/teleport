/*
Copyright 2023 Gravitational, Inc.

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

package utils

import (
	"golang.org/x/crypto/bcrypt"
)

const maxInputSize = 72

// truncateToMaxSize Make sure input is truncated to the maximum length crypto accepts.  Crypto changed the behavior
// from ignoring the extra input to returning an error, this truncation is necessary to maintain compatibility with
// customers who have long passwords, or more commonly our recovery codes.
func truncateToMaxSize(input []byte) []byte {
	if len(input) > maxInputSize {
		return input[:maxInputSize]
	}
	return input
}

// BcryptFromPassword delegates to bcrypt.GenerateFromPassword, but maintains the prior behavior of only hashing the
// first 72 bytes.  BCrypt as an algorithm can not hash inputs > 72 bytes.
func BcryptFromPassword(password []byte, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword(truncateToMaxSize(password), cost)
}
