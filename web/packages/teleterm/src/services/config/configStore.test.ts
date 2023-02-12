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

import { z } from 'zod';

import Logger, { NullService } from 'teleterm/logger';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

import { createConfigStore } from './configStore';

beforeAll(() => {
  Logger.init(new NullService());
});

const schema = z.object({
  'fonts.monoFamily': z.string().default('Arial'),
  'usageReporting.enabled': z.boolean().default(false),
});

test('stored and default values are combined', () => {
  const fileStorage = createMockFileStorage();
  fileStorage.put('usageReporting.enabled', true);
  const configStore = createConfigStore(schema, fileStorage);

  expect(configStore.getStoredConfigErrors()).toBeUndefined();

  const usageReportingEnabled = configStore.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(true);
  expect(usageReportingEnabled.metadata.isStored).toBe(true);

  const monoFontFamily = configStore.get('fonts.monoFamily');
  expect(monoFontFamily.value).toBe('Arial');
  expect(monoFontFamily.metadata.isStored).toBe(false);
});

test('in case of invalid value a default one is returned', () => {
  const fileStorage = createMockFileStorage();
  fileStorage.put('usageReporting.enabled', 'abcde');
  const configStore = createConfigStore(schema, fileStorage);

  expect(configStore.getStoredConfigErrors()).toStrictEqual([
    {
      code: 'invalid_type',
      expected: 'boolean',
      received: 'string',
      message: 'Expected boolean, received string',
      path: ['usageReporting.enabled'],
    },
  ]);

  const usageReportingEnabled = configStore.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(false);
  expect(usageReportingEnabled.metadata.isStored).toBe(false);

  const monoFontFamily = configStore.get('fonts.monoFamily');
  expect(monoFontFamily.value).toBe('Arial');
  expect(monoFontFamily.metadata.isStored).toBe(false);
});

test('calling set updated the value in store', () => {
  const fileStorage = createMockFileStorage();
  const configStore = createConfigStore(schema, fileStorage);

  configStore.set('usageReporting.enabled', true);

  const usageReportingEnabled = configStore.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(true);
  expect(usageReportingEnabled.metadata.isStored).toBe(true);
});
