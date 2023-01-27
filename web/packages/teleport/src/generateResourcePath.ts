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

/* The generatePath function built into react router doesn't use encodeURIComponent. This causes the resulting encoded URL to be
encoded improperly for this use-case, and the query fails. This custom generateResourcePath function resolves that issue while also
being more specialized to this use-case and supporting a SortType param.

Example:

Output from generatePath: /v1/webapi/sites/im-a-cluster-name/apps?limit=&startKey=&query=labels.app%20==%20%22banana%22&search=&sort=
Output from generateResourcePath: /v1/webapi/sites/im-a-cluster-name/apps?limit=5&startKey=&query=labels.app%20%3D%3D%20%22banana%22&search=&sort=name:asc

*/

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
    } else {
      processedParams[param] = params[param]
        ? encodeURIComponent(params[param])
        : '';
    }
  }
  const output = path
    .replace(':clusterId', params.clusterId)
    .replace(':limit?', params.limit)
    .replace(':startKey?', params.startKey || '')
    .replace(':query?', processedParams.query || '')
    .replace(':search?', processedParams.search || '')
    .replace(':sort?', processedParams.sort || '');

  return output;
}
