/**
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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// canonicalKeyFixture matches the JSON shape of testdata/canonical-key-fixtures.json.
// The file is also consumed by e2e/scripts/verify-canonical-key.ts, which exercises
// the TypeScript implementation against the same inputs. Both implementations must
// produce byte-identical output; drift is the whole thing this test exists to catch.
type canonicalKeyFixture struct {
	Name     string            `json:"name"`
	Input    canonicalKeyInput `json:"input"`
	Expected string            `json:"expected"`
}

type canonicalKeyInput struct {
	Roles  []json.RawMessage   `json:"roles"`
	Traits map[string][]string `json:"traits,omitempty"`
}

const fileRolePrefix = "@gravitational/e2e/roles/"

func TestCanonicalUserKeyFixtures(t *testing.T) {
	path := filepath.Join("..", "testdata", "canonical-key-fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var fixtures []canonicalKeyFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}

	if len(fixtures) == 0 {
		t.Fatal("no fixtures found")
	}

	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			su, err := scannedUserFromFixture(f.Input)
			if err != nil {
				t.Fatalf("decode input: %v", err)
			}

			got := canonicalUserKey(su)
			if got != f.Expected {
				t.Errorf("canonicalUserKey mismatch\ninput:    %s\nexpected: %s\ngot:      %s",
					marshalJSON(t, f.Input), f.Expected, got)
			}
		})
	}
}

// scannedUserFromFixture converts the TypeScript-shaped fixture input (where
// file roles are quoted with the @gravitational/e2e/roles/ prefix) into the Go
// scannedUser form (which strips the prefix at scan time).
func scannedUserFromFixture(in canonicalKeyInput) (scannedUser, error) {
	var roles []scannedRole
	for _, raw := range in.Roles {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			roles = append(roles, scannedRole{name: s})
			continue
		}

		var f struct {
			File string `json:"file"`
		}

		if err := json.Unmarshal(raw, &f); err != nil {
			return scannedUser{}, err
		}

		roles = append(roles, scannedRole{
			file: strings.TrimPrefix(f.File, fileRolePrefix),
		})
	}

	return scannedUser{roles: roles, traits: in.Traits}, nil
}

func marshalJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return string(b)
}
