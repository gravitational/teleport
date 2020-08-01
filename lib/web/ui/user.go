/**
 * Copyright 2020 Gravitational, Inc.
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

package ui

import "time"

// User contains data needed by the web UI to display locally saved users.
type User struct {
	// Name is a user name.
	Name string `json:"name"`
	// Roles is a list of user roles.
	Roles []string `json:"roles"`
	// Created is user creation time.
	Created time.Time `json:"created"`
}

// UserToken contains data on newly created user and password setup token.
type UserToken struct {
	// Name of who token belongs to
	Name string `json:"name"`
	// URL is the link to setup password for new user.
	URL string `json:"url"`
	// Created is when the token was created.
	Created time.Time `json:"created"`
	// Expires is when token expires.
	Expires time.Time `json:"expires"`
}
