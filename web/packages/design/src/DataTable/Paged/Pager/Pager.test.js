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
import Pager from './Pager';
import { render } from 'design/utils/testing';

describe('design/DataTable Pager', () => {
  test.each`
    startFrom | endAt | totalRows | expPrevNextBtns   | expNumRanges
    ${2}      | ${5}  | ${10}     | ${[false, false]} | ${[3, 5, 10]}
    ${0}      | ${5}  | ${10}     | ${[true, false]}  | ${[1, 5, 10]}
    ${1}      | ${5}  | ${5}      | ${[false, true]}  | ${[2, 5, 5]}
    ${0}      | ${0}  | ${0}      | ${[true, true]}   | ${[0, 0, 0]}
  `(
    'respects props: startFrom=$startFrom, endAt=$endAt, totalRows=$totalRows, disablePrevNext=$expPrevNextBtns',
    ({ startFrom, endAt, totalRows, expPrevNextBtns, expNumRanges }) => {
      const mockFn = jest.fn();
      const { container } = render(
        <Pager
          startFrom={startFrom}
          endAt={endAt}
          totalRows={totalRows}
          onPrev={mockFn}
          onNext={mockFn}
        />
      );

      expect(container.firstChild.textContent).toEqual(
        `SHOWING ${expNumRanges[0]} - ${expNumRanges[1]} of ${expNumRanges[2]}`
      );

      const buttons = container.querySelectorAll('button');
      expect(buttons[0].disabled).toBe(expPrevNextBtns[0]);
      expect(buttons[1].disabled).toBe(expPrevNextBtns[1]);
    }
  );
});
