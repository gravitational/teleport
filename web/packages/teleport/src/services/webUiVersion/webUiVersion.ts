/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { ApiError } from '../api/parseError';

export function getWebUiVersion() {
  const metaTag = document.querySelector<HTMLMetaElement>(
    '[name=teleport_version]'
  );
  return metaTag?.content || '';
}

export function withUnsupportedLabelFeatureErrorConversion(
  err: unknown
): never {
  if (err instanceof ApiError && err.response.status === 404) {
    if (err.proxyVersion && err.proxyVersion.string) {
      throw new Error(
        'We could not complete your request. ' +
          `Your proxy (v${err.proxyVersion.string}) may be behind the minimum required version ` +
          `(v17.2.0) to support adding resource labels. ` +
          'Ensure all proxies are upgraded or remove labels and try again.'
      );
    }

    throw new Error(
      'We could not complete your request. ' +
        'Your proxy may be behind the minimum required version ' +
        `(v17.2.0) to support adding resource labels. ` +
        'Ensure all proxies are upgraded or remove labels and try again.'
    );
  }
  throw err;
}
