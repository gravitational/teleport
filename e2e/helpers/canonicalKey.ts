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

// UserKeyInput is the subset of UserDefinition that participates in the
// canonical key. Kept here (rather than importing from test.ts) so the
// parity verification script can load it without pulling in Playwright.
export type UserKeyInput = {
  roles: (string | { file: string })[];
  traits?: Record<string, string[] | undefined>;
};

/**
 * Produces a canonical key for a user definition. MUST match the output of
 * the Go runner's `canonicalUserKey` in e2e/runner/bootstrap.go — if you
 * change this function, change that one too and regenerate the fixtures in
 * testdata/canonical-key-fixtures.json.
 */
export function canonicalUserKey(def: UserKeyInput): string {
  const roles = def.roles
    .map(r =>
      typeof r === 'string'
        ? r
        : `@file:${r.file.replace('@gravitational/e2e/roles/', '')}`
    )
    .sort();

  const result: { roles: string[]; traits?: Record<string, string[]> } = {
    roles,
  };

  if (def.traits) {
    const sorted: Record<string, string[]> = {};
    for (const k of Object.keys(def.traits).sort()) {
      const v = def.traits[k];
      if (v) {
        sorted[k] = [...v].sort();
      }
    }

    if (Object.keys(sorted).length > 0) {
      result.traits = sorted;
    }
  }

  return JSON.stringify(result);
}
