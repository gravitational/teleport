/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
    .replace(':limit?', params.limit || '')
    .replace(':startKey?', params.startKey || '')
    .replace(':query?', processedParams.query || '')
    .replace(':search?', processedParams.search || '')
    .replace(':searchAsRoles?', processedParams.searchAsRoles || '')
    .replace(':sort?', processedParams.sort || '')
    .replace(':kinds?', processedParams.kinds || '')
    .replace(':pinnedOnly?', processedParams.pinnedOnly || '');

  return output;
}
