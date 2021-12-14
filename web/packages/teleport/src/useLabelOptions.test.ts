/**
 * Copyright 2021 Gravitational, Inc.
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

import renderHook, { act } from 'design/utils/renderHook';
import { Label } from 'teleport/types';
import { makeLabelTag } from 'teleport/components/formatters';
import useLabelOptions, { Data, State } from './useLabelOptions';

test('correct formatting of options from list of data and labels', () => {
  let result;
  act(() => {
    result = renderHook(() => useLabelOptions(data, [label1, label2]));
  });

  let options: State = result.current;

  expect(options.all).toHaveLength(2);
  expect(options.selected).toHaveLength(2);

  // Test sort and format of a option.
  expect(options.all[0].value).toEqual(makeLabelTag(label2));
  expect(options.all[0].label).toEqual(makeLabelTag(label2));
  expect(options.all[0].obj).toMatchObject(label2);

  // Test format of a selected option.
  expect(options.selected[0].value).toEqual(makeLabelTag(label1));
  expect(options.selected[0].label).toEqual(makeLabelTag(label1));
  expect(options.selected[0].obj).toMatchObject(label1);
});

const label1: Label = { name: 'key80', value: 'value80' };
const label2: Label = { name: 'key9', value: 'value9' };
const data: Data[] = [{ labels: [label1] }, { labels: [label1, label2] }];
