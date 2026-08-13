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

// The proxy's rate limiter buckets by client IP, so a whole run sharing one address lets setup logins and
// unauthenticated specs exhaust each other's budget. With trust_x_forwarded_for enabled on the test cluster,
// every browser context claims its own address instead.
//
// IPv6 is required, not cosmetic: the proxy signs a PROXY header pairing this source with the destination of
// the browser's connection, which is ::1 because the tests reach the proxy over localhost, and signing rejects
// a version mismatch. The runner hands bootstrapped users fd00:e2e:1:: (assignClientIP in
// runner/usercredentials.go), so per-test addresses sit under a different prefix.
const TEST_PREFIX = 'fd00:e2e:2';

// The hash fills the two trailing groups, giving 32 bits of address per test.
export function testClientIp(testId: string) {
  const h = fnv1a(testId);

  const hi = (h >>> 16).toString(16);
  const lo = (h & 0xffff).toString(16);

  return `${TEST_PREFIX}:${hi}::${lo}`;
}

// fnv1a generates a short string hash for a test ID.
function fnv1a(s: string) {
  let h = 0x811c9dc5;

  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }

  return h >>> 0;
}
