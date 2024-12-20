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
  err: any
): Promise<any> {
  if (err instanceof ApiError) {
    if (err.response.status === 404) {
      throw new Error(
        'We could not complete your request. ' +
          'Your proxy may be behind the minimum required version ' +
          `(${getWebUiVersion()}) to support adding resource labels. ` +
          'Upgrade your proxy version or remove labels and try again.'
      );
    }
  }
  throw err;
}
