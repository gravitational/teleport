/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

const BASE_PATH = '/web';

/**
 * Processes a redirect URI to ensure it's valid and follows the expected format.
 *
 * This function handles various cases:
 * - Null or empty input: Returns the basePath
 * - Full URLs:
 *   - External: Uses only the pathname, prepending the basePath if not already present
 *   - Internal: Prepends the basePath if not already present
 * - Relative paths: Prepends the basePath if not already present
 * - Invalid URIs: Returns the basePath
 *
 * @param redirectUri - The redirect URI to process. Can be null, a relative path, or a full URL.
 * @returns A processed URI string that always starts with the basePath, unless it's an invalid input.
 *
 * @example
 * processRedirectURI('/web', null) // returns '/web'
 * processRedirectURI('/web', 'https://example.com/path') // returns '/web/path'
 * processRedirectURI('/web', '/custom/path') // returns '/web/custom/path'
 * processRedirectURI('/web', '/web/existing/path') // returns '/web/existing/path'
 * processRedirectURI('/web', 'invalid://url') // returns '/web'
 * processRedirectURI('/app', 'https://example.com/path') // returns '/app/path'
 */
export function processRedirectUri(redirectUri: string | null): string {
  // should be equal to cfg.routes.root
  if (!redirectUri) {
    return BASE_PATH;
  }
  try {
    const url = new URL(redirectUri);
    const path = url.pathname;
    // If it already starts with basePath, return as is
    if (path.startsWith(BASE_PATH)) {
      return path;
    }

    if (path === '/') {
      return BASE_PATH;
    }

    return `${BASE_PATH}${path.startsWith('/') ? '' : '/'}${path}`;
  } catch {
    // If it's not a valid URL, it might be a relative path
    if (redirectUri.startsWith('/')) {
      return redirectUri.startsWith(BASE_PATH)
        ? redirectUri
        : `${BASE_PATH}${redirectUri}`;
    }
    // For invalid URIs, return the default
    return BASE_PATH;
  }
}
