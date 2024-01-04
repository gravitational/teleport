/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export function makeAdvancedSearchQueryForLabel(
  label: {
    name: string;
    value: string;
  },
  params: {
    /** Contains search words/phrases separated by space. */
    search?: string;
    /** Query expression using the predicate language. */
    query?: string;
  }
): string {
  const queryParts: string[] = [];

  // Add an existing query.
  if (params.query) {
    queryParts.push(params.query);
  }

  // If there is an existing simple search, convert it to predicate language and add it.
  if (params.search) {
    queryParts.push(`search("${params.search}")`);
  }

  const labelQuery = `labels["${label.name}"] == "${label.value}"`;
  queryParts.push(labelQuery);

  return queryParts.join(' && ');
}
