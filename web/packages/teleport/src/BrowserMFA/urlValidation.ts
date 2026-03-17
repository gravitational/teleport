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

export function validateClientRedirect(url: string): string {
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

  return url;
}
