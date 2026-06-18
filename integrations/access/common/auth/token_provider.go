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

package auth

// AccessTokenProvider provides a method to get the bearer token
// for use when authorizing to a 3rd-party provider API.
type AccessTokenProvider interface {
	GetAccessToken() (string, error)
}

// StaticAccessTokenProvider is an implementation of AccessTokenProvider
// that always returns the specified token.
type StaticAccessTokenProvider struct {
	token string
}

// NewStaticAccessTokenProvider creates a new StaticAccessTokenProvider.
func NewStaticAccessTokenProvider(token string) *StaticAccessTokenProvider {
	return &StaticAccessTokenProvider{token: token}
}

// GetAccessToken implements AccessTokenProvider
func (s *StaticAccessTokenProvider) GetAccessToken() (string, error) {
	return s.token, nil
}
