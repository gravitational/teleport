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
const SAML_SP_INITIATED_SSO_PATH = '/enterprise/saml-idp/sso';
const SAML_IDP_INITIATED_SSO_PATH = '/enterprise/saml-idp/login';

/**
 * Processes a redirect URI to ensure it's valid and follows the expected format.
 *
 * This function handles various cases:
 * - Null or empty input: Returns the base path ('/web')
 * - Full URLs: Skips the host and scheme, prepending the base path if not already present
 * - Relative paths: Prepends the base path if not already present
 * - Invalid URIs: Returns the base path
 *
 * Nobody seems to know why we need to prepend the base path to the URL, so we keep doing it. It
 * might be related to the URLs we get from SSO redirects [1], but it's unclear why we'd be getting
 * a URL that's missing the base path and becomes valid only after appending the base path.
 *
 * [1]: https://github.com/gravitational/teleport/pull/47221#discussion_r1792248868
 *
 * You might ask: why is the browser doing this work if it's then repeated in getRedirectURL in
 * lib/web/device_trust.go. The answer is that the backend does it only when the authorization is
 * successful. If the user opts to skip Device Trust, we still need to be able to get them to the
 * redirect URL using the same logic.
 *
 * @param redirectUri - The redirect URI to process. Can be null, a relative path, or a full URL.
 * @returns A relative path that always starts with the base path, unless it's a SAML redirect then
 * the base path is not added.
 *
 * @example
 * processRedirectURI(null) // returns '/web'
 * processRedirectURI('https://example.com/path') // returns '/web/path'
 * processRedirectURI('/custom/path') // returns '/web/custom/path'
 * processRedirectURI('/web/existing/path') // returns '/web/existing/path'
 * processRedirectURI('invalid://url') // returns '/web'
 */
export function processRedirectUri(redirectUri: string | null): string {
  // should be equal to cfg.routes.root
  if (!redirectUri) {
    return BASE_PATH;
  }

  // Handle relative paths first.
  if (redirectUri.startsWith('/')) {
    return redirectUri.startsWith(BASE_PATH)
      ? redirectUri
      : `${BASE_PATH}${redirectUri}`;
  }

  let url: URL;
  try {
    url = new URL(redirectUri);
  } catch {
    return BASE_PATH;
  }

  // Do not prepend BASE_PATH to SAML redirects.
  let path = url.pathname;
  if (
    path.startsWith(SAML_IDP_INITIATED_SSO_PATH) ||
    path.startsWith(SAML_SP_INITIATED_SSO_PATH)
  ) {
    return path + url.search;
  }

  if (path === '/') {
    path = BASE_PATH;
  } else if (!path.startsWith(BASE_PATH)) {
    path = `${BASE_PATH}${path.startsWith('/') ? '' : '/'}${path}`;
  }

  return path + url.search;
}
