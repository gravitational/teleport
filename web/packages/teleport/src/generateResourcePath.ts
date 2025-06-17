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
    } else if (param === 'statuses') {
      processedParams[param] = (params[param] ?? []).join('&status=');
    } else if (param === 'regions') {
      processedParams[param] = (params[param] ?? []).join('&regions=');
    } else
      processedParams[param] = params[param]
        ? encodeURIComponent(params[param])
        : '';
  }

  // as of now, "none" and "all" function the same. if both options are selected (requestable, accessible_only)
  // then you will see the same results. The distinction comes from user preferences, which change the visual of
  // the filter. If "none", there are no options selected. If "all", both options are selected and a filter indicator
  // is shown.
  if (processedParams.includedResourceMode === 'none') {
    processedParams.includedResourceMode = 'all';
  }

  const output = path
    // non-param
    .replace(':clusterId', params.clusterId)
    .replace(':name', params.name || '')
    // param
    .replace(':kind?', processedParams.kind || '')
    .replace(':kinds?', processedParams.kinds || '')
    .replace(':status?', processedParams.statuses || '')
    .replace(':kubeCluster?', processedParams.kubeCluster || '')
    .replace(':kubeNamespace?', processedParams.kubeNamespace || '')
    .replace(':limit?', params.limit || '')
    .replace(':pinnedOnly?', processedParams.pinnedOnly || '')
    .replace(':query?', processedParams.query || '')
    .replace(':resourceType?', params.resourceType || '')
    .replace(':search?', processedParams.search || '')
    .replace(':searchAsRoles?', processedParams.searchAsRoles || '')
    .replace(':sort?', processedParams.sort || '')
    .replace(':startKey?', params.startKey || '')
    .replace(':regions?', processedParams.regions || '')
    .replace(
      ':includedResourceMode?',
      processedParams.includedResourceMode || ''
    );

  return output;
}
