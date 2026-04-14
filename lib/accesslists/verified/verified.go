//go:build verified_accesslists

/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package verified provides a formally verified implementation of access list
// membership checking, implemented in Rust and translated to Lean4 for theorem
// proving via Aeneas.
package verified

/*
// Flags to include the static Rust library.
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../target/x86_64-unknown-linux-gnu/release
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../target/aarch64-unknown-linux-gnu/release
#cgo linux LDFLAGS: -l:libaccesslist_verify.a -lpthread -ldl -lm
#cgo darwin,amd64 LDFLAGS: -L${SRCDIR}/../../../target/x86_64-apple-darwin/release
#cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/../../../target/aarch64-apple-darwin/release
#cgo darwin LDFLAGS: -laccesslist_verify -lpthread -ldl -lm
#include "rust/libaccesslist_verify.h"
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
)

// traitEntry mirrors the Rust TraitEntry struct for JSON serialization.
type traitEntry struct {
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// userInfo mirrors the Rust UserInfo struct for JSON serialization.
type userInfo struct {
	Roles  []string     `json:"roles"`
	Traits []traitEntry `json:"traits"`
}

// ffiInput is the JSON structure expected by the Rust FFI function.
type ffiInput struct {
	User     userInfo `json:"user"`
	Requires userInfo `json:"requires"`
}

// UserMeetsRequirements checks whether the user meets the given access list
// requirements using the formally verified Rust implementation.
//
// This function is equivalent to accesslists.UserMeetsRequirements in
// lib/accesslists/hierarchy.go, but calls into the Rust implementation
// that has been formally verified via Lean4.
func UserMeetsRequirements(identity types.User, requires accesslist.Requires) (bool, error) {
	// Convert user traits to trait entries.
	userTraits := make([]traitEntry, 0)
	for k, values := range identity.GetTraits() {
		userTraits = append(userTraits, traitEntry{Key: k, Values: values})
	}

	// Convert required traits to trait entries.
	reqTraits := make([]traitEntry, 0)
	for k, values := range requires.Traits {
		reqTraits = append(reqTraits, traitEntry{Key: k, Values: values})
	}

	userRoles := identity.GetRoles()
	if userRoles == nil {
		userRoles = []string{}
	}
	reqRoles := requires.Roles
	if reqRoles == nil {
		reqRoles = []string{}
	}

	input := ffiInput{
		User: userInfo{
			Roles:  userRoles,
			Traits: userTraits,
		},
		Requires: userInfo{
			Roles:  reqRoles,
			Traits: reqTraits,
		},
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return false, trace.Wrap(err, "marshaling FFI input")
	}

	cStr := C.CString(string(jsonBytes))
	defer C.free(unsafe.Pointer(cStr))

	result := C.verified_user_meets_requirements(cStr)
	switch result {
	case 1:
		return true, nil
	case 0:
		return false, nil
	default:
		return false, trace.Errorf("Rust FFI returned error code %d", result)
	}
}
