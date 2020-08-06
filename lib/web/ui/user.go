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
	// Name is the user name.
	Name string `json:"name"`
	// Roles is the list of roles user belongs to.
	Roles []string `json:"roles"`
	// Created is when user was created.
	Created time.Time `json:"created"`
}

// ResetToken contains data about user password setup.
type ResetToken struct {
	// Name of who token belongs to.
	Name string `json:"name"`
	// Value is the token ID used to construct password reset URL.
	Value string `json:"value"`
	// Expires is when token expires in format HourMinuteSeconds.
	Expires string `json:"expires"`
}
