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

import { usePages } from '.';
import renderHook, { act } from 'design/utils/renderHook';

describe('design/DataTable usePages hook', () => {
  let pager = null;
  const data = [1, 2, 3, 4, 5, 6, 7, 8, 9];
  const pageSize = 2;

  beforeEach(() => {
    pager = renderHook(() => usePages({ pageSize, data }));
  });

  // test out of bounds on both onPrev() and onNext()
  // test correct incrementing onNext()
  // test correct decrementing onPrev()
  test.each`
    numPrevCalls | expStartFrom | expEndAt | expData   | numNextCalls
    ${1}         | ${0}         | ${2}     | ${[1, 2]} | ${0}
    ${1}         | ${0}         | ${2}     | ${[1, 2]} | ${1}
    ${1}         | ${6}         | ${8}     | ${[7, 8]} | ${4}
    ${4}         | ${0}         | ${2}     | ${[1, 2]} | ${4}
    ${0}         | ${2}         | ${4}     | ${[3, 4]} | ${1}
    ${0}         | ${4}         | ${6}     | ${[5, 6]} | ${2}
    ${0}         | ${6}         | ${8}     | ${[7, 8]} | ${3}
    ${0}         | ${8}         | ${9}     | ${[9]}    | ${4}
    ${0}         | ${8}         | ${9}     | ${[9]}    | ${5}
  `(
    '$numNextCalls x onNext() with $numPrevCalls x onPrev(): startFrom = $expStartFrom, endAt = $expEndAt, paged data = $expData',
    ({ numPrevCalls, expStartFrom, expEndAt, expData, numNextCalls }) => {
      for (let i = 0; i < numNextCalls; i += 1) {
        act(() => pager.current.onNext());
      }

      for (let i = 0; i < numPrevCalls; i += 1) {
        act(() => pager.current.onPrev());
      }

      expect(pager.current.pageSize).toBe(pageSize);
      expect(pager.current.totalRows).toBe(data.length);
      expect(pager.current.hasPages).toBe(true);
      expect(pager.current.startFrom).toBe(expStartFrom);
      expect(pager.current.data).toEqual(expData);
      expect(pager.current.endAt).toBe(expEndAt);
    }
  );
});
