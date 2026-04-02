/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

const ALLOWED_HOSTS = ['localhost', '127.0.0.1', '[::1]'];

/**
 * Validates that a redirect URL points to a valid local address, uses an
 * allowed protocol, and does not expose URL-based credentials.
 *
 * This function is intended for validating client-side redirect URLs used in
 * flows such as browser MFA, where the redirect target must be a local address
 * (e.g. tsh, tctl etc) rather than an arbitrary external URL.
 *
 * Throws an error if any of the following conditions are not met:
 * - The URL must not be empty.
 * - The hostname must be one of the allowed local hosts: localhost, 127.0.0.1, or [::1].
 * - The protocol must be http: or https:.
 * - The URL must not contain a username or password.
 *
 * @param url - The redirect URL string to validate.
 * @throws {Error} If the URL is empty, has a disallowed host, uses an invalid protocol,
 * or contains embedded credentials.
 *
 * @example
 * validateClientRedirect('http://localhost:8080/callback') // valid, does not throw
 * validateClientRedirect('http://evil.com/callback')       // throws: not a valid local address
 * validateClientRedirect('ftp://localhost/callback')       // throws: not a valid protocol
 * validateClientRedirect('http://user:pass@localhost/')    // throws: must not contain credentials
 */
export function validateClientRedirect(url: string) {
  if (url === '') {
    throw new Error('redirect URL must not be empty');
  }

  const parsedUrl = new URL(url);

  if (!ALLOWED_HOSTS.includes(parsedUrl.hostname)) {
    throw new Error(`${parsedUrl.hostname} is not a valid local address`);
  }

  if (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:') {
    throw new Error(`${parsedUrl.protocol} is not a valid protocol`);
  }

  if (parsedUrl.username || parsedUrl.password) {
    throw new Error('redirect URL must not contain credentials');
  }
}
