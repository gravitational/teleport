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

import audit from './audit';
import api from 'teleport/services/api';
import * as makeEventObject from './makeEvent';

test('fetch latest events', async () => {
  // Test null response gives empty array.
  jest.spyOn(api, 'get').mockResolvedValue({ events: null });

  let response = await audit.fetchLatest();

  expect(api.get).toHaveBeenCalledTimes(1);
  expect(response.events).toHaveLength(0);
  expect(response.overflow).toEqual(false);

  // Test events overflow.
  jest
    .spyOn(api, 'get')
    .mockResolvedValue({ events: Array(audit.maxLimit + 1) });
  jest.spyOn(makeEventObject, 'default').mockReturnValue(null as any);

  response = await audit.fetchLatest();

  expect(response.events).toHaveLength(audit.maxLimit - 1);
  expect(response.overflow).toEqual(true);
});
