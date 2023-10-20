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

import Logger, { NullService } from 'teleterm/logger';
import { createMockFileStorage } from 'teleterm/services/fileStorage/fixtures/mocks';

import { createConfigService } from './configService';

beforeAll(() => {
  Logger.init(new NullService());
});

test('stored and default values are combined', () => {
  const configFile = createMockFileStorage();
  configFile.put('usageReporting.enabled', true);
  const configService = createConfigService({
    configFile,
    jsonSchemaFile: createMockFileStorage(),
    platform: 'darwin',
  });

  expect(configService.getConfigError()).toBeUndefined();

  const usageReportingEnabled = configService.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(true);
  expect(usageReportingEnabled.metadata.isStored).toBe(true);

  const terminalFontSize = configService.get('terminal.fontSize');
  expect(terminalFontSize.value).toBe(15);
  expect(terminalFontSize.metadata.isStored).toBe(false);
});

test('in case of invalid value a default one is returned', () => {
  const configFile = createMockFileStorage();
  configFile.put('usageReporting.enabled', 'abcde');
  const configService = createConfigService({
    configFile: configFile,
    jsonSchemaFile: createMockFileStorage(),
    platform: 'darwin',
  });

  expect(configService.getConfigError()).toStrictEqual({
    source: 'validation',
    errors: [
      {
        code: 'invalid_type',
        expected: 'boolean',
        received: 'string',
        message: 'Expected boolean, received string',
        path: ['usageReporting.enabled'],
      },
    ],
  });

  const usageReportingEnabled = configService.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(false);
  expect(usageReportingEnabled.metadata.isStored).toBe(false);

  const terminalFontSize = configService.get('terminal.fontSize');
  expect(terminalFontSize.value).toBe(15);
  expect(terminalFontSize.metadata.isStored).toBe(false);
});

test('if config file failed to load correctly the error is returned', () => {
  const configFile = createMockFileStorage();
  const error = new Error('Failed to read');
  jest.spyOn(configFile, 'getFileLoadingError').mockReturnValue(error);

  const configService = createConfigService({
    configFile,
    jsonSchemaFile: createMockFileStorage(),
    platform: 'darwin',
  });

  expect(configService.getConfigError()).toStrictEqual({
    source: 'file-loading',
    error,
  });
});

test('calling set updated the value in store', () => {
  const configFile = createMockFileStorage();
  const configService = createConfigService({
    configFile,
    jsonSchemaFile: createMockFileStorage(),
    platform: 'darwin',
  });

  configService.set('usageReporting.enabled', true);

  const usageReportingEnabled = configService.get('usageReporting.enabled');
  expect(usageReportingEnabled.value).toBe(true);
  expect(usageReportingEnabled.metadata.isStored).toBe(true);
});

test('field linking to the json schema and the json schema itself are updated', () => {
  const configFile = createMockFileStorage();
  const jsonSchemaFile = createMockFileStorage({
    filePath: '~/config_schema.json',
  });

  jest.spyOn(jsonSchemaFile, 'replace');

  createConfigService({
    configFile,
    jsonSchemaFile,
    platform: 'darwin',
  });

  expect(configFile.get('$schema')).toBe('config_schema.json');
  expect(jsonSchemaFile.replace).toHaveBeenCalledTimes(1);
});
