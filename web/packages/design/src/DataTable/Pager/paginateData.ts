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

// paginateData breaks the data array up into chunks the length of pageSize
export default function paginateData(
  data = [],
  pageSize = 10
): Array<Array<any>> {
  const pageCount = Math.ceil(data.length / pageSize);
  const pages = [];

  for (let i = 0; i < pageCount; i++) {
    const start = i * pageSize;
    const page = data.slice(start, start + pageSize);
    pages.push(page);
  }

  // If there are no items, place an empty page inside pages
  if (pages.length === 0) {
    pages[0] = [];
  }

  return pages;
}
