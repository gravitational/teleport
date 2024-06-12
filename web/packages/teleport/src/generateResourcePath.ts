/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

// generateResourcePath constructs the agent endpoint URL using `encodeURIComponent`.
// react-router's `generatePath` uses `encodeURI` which does not encode the entire string.

export default function generateResourcePath(
  path: string,
  params?: {
    [x: string]: any;
  }
) {
  const processedParams: typeof params = {};
  for (const param in params) {
    // If the param is SortType, turn it into string with "[param.fieldName]:[param.dir]"
    if (params[param]?.dir) {
      processedParams[param] = `${params[param].fieldName}:${params[
        param
      ].dir.toLowerCase()}`;
    } else if (param === 'kinds') {
      processedParams[param] = (params[param] ?? []).join('&kinds=');
    } else
      processedParams[param] = params[param]
        ? encodeURIComponent(params[param])
        : '';
  }

  const output = path
    .replace(':clusterId', params.clusterId)
    .replace(':limit?', params.limit)
    .replace(':startKey?', params.startKey || '')
    .replace(':query?', processedParams.query || '')
    .replace(':search?', processedParams.search || '')
    .replace(':searchAsRoles?', processedParams.searchAsRoles || '')
    .replace(':sort?', processedParams.sort || '')
    .replace(':kinds?', processedParams.kinds || '')
    .replace(':pinnedOnly?', processedParams.pinnedOnly || '');

  return output;
}
