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

// Runs the TypeScript canonicalUserKey over the shared fixture file and
// asserts each input produces the expected key. Exits non-zero on any
// mismatch. The Go runner exercises the same fixture in
// e2e/runner/canonical_key_test.go; both implementations must agree byte-for-byte.

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { canonicalUserKey, type UserKeyInput } from '../helpers/canonicalKey';

type Fixture = {
  name: string;
  input: UserKeyInput;
  index?: number;
  isDefault?: boolean;
  source?: string;
  expected: string;
};

const fixturesPath = join(
  dirname(fileURLToPath(import.meta.url)),
  '../testdata/canonical-key-fixtures.json'
);
const fixtures: Fixture[] = JSON.parse(readFileSync(fixturesPath, 'utf-8'));

let failed = 0;
for (const { name, input, index, isDefault, source, expected } of fixtures) {
  const got = canonicalUserKey(input, { index, isDefault, source });
  if (got === expected) {
    console.log(`ok - ${name}`);
    continue;
  }
  failed++;
  console.error(`FAIL - ${name}`);
  console.error(`  input:    ${JSON.stringify(input)}`);
  console.error(`  opts:     ${JSON.stringify({ index, isDefault, source })}`);
  console.error(`  expected: ${expected}`);
  console.error(`  got:      ${got}`);
}

if (failed > 0) {
  console.error(`\n${failed}/${fixtures.length} fixture(s) failed`);
  process.exit(1);
}

console.log(`\n${fixtures.length} fixture(s) verified`);
