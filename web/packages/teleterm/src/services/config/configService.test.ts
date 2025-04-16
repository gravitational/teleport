/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import Logger, { NullService } from 'teleterm/logger';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
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
    settings: makeRuntimeSettings(),
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
    settings: makeRuntimeSettings(),
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
    settings: makeRuntimeSettings(),
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
    settings: makeRuntimeSettings(),
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
    settings: makeRuntimeSettings(),
  });

  expect(configFile.get('$schema')).toBe('config_schema.json');
  expect(jsonSchemaFile.replace).toHaveBeenCalledTimes(1);
});
