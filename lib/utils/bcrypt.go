/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
