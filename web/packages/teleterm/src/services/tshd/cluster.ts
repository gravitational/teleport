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

import * as whatwg from 'whatwg-url';

/**
 * Accepts a proxy host in the form of "cluster-address.example.com:3090" and returns the host as
 * understood by browsers.
 *
 * The URL API in most browsers skips the port if the port matches the default port used by the
 * protocol. This behavior can be observed both in JS and in DOM. For example:
 *
 *     <a href="https://example.com:443/hello-world">Example</a>
 *
 * becomes
 *
 *     <a href="https://example.com/hello-world">Example</a>
 *
 * The distinction is important in situations where we want to match the host reported by the
 * browser against a host that we got from a Go service.
 */
export function proxyHostToBrowserProxyHost(proxyHost: string) {
  let whatwgURL: whatwg.URL;

  try {
    whatwgURL = new whatwg.URL(`https://${proxyHost}`);
  } catch (error) {
    if (error instanceof TypeError) {
      throw new Error(`Invalid proxy host ${proxyHost}`, { cause: error });
    }
    throw error;
  }

  // Catches cases where proxyHost itself includes a "https://" prefix.
  if (whatwgURL.pathname !== '/') {
    throw new Error(`Invalid proxy host ${proxyHost}`);
  }

  return whatwgURL.host;
}
