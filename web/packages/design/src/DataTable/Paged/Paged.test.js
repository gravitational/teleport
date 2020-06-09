/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import TablePaged from './Paged';
import { render } from 'design/utils/testing';

const data = [1, 2, 3, 4, 5, 6, 7];
const pageSize = 2;

test('pagerPosition set to bottom', () => {
  let { container } = render(
    <TablePaged pageSize={pageSize} data={data} pagerPosition={'bottom'} />
  );
  expect(container.firstChild.children[1].nodeName).toEqual('NAV');
});

test('pagerPosition set to top', () => {
  let { container } = render(
    <TablePaged pageSize={pageSize} data={data} pagerPosition={'top'} />
  );
  expect(container.firstChild.children[0].nodeName).toEqual('NAV');
});

test('pagerPosition prop default (show only top)', () => {
  let { container } = render(<TablePaged pageSize={pageSize} data={data} />);
  expect(container.querySelectorAll('nav')).toHaveLength(1);
});
