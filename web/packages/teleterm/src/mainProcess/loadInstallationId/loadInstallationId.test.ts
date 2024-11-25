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

import crypto from 'crypto';
import fs from 'fs';

import { loadInstallationId } from './loadInstallationId';

jest.mock('fs');
jest.mock('crypto');

beforeEach(() => {
  jest.resetAllMocks();
});

test('returns ID stored on disk', () => {
  const storedId = '0026d6e2-d9dd-409f-a972-f8ec056a636c';
  jest.spyOn(fs, 'readFileSync').mockReturnValue(storedId);

  expect(loadInstallationId('')).toBe(storedId);
});

test('generates a new ID if reading it from disk causes an error', () => {
  const newId = '0026d6e2-d9dd-409f-a972-f8ec056a636c';
  const filePath = '/test_path';
  jest.spyOn(crypto, 'randomUUID').mockReturnValue(newId);
  jest.spyOn(fs, 'readFileSync').mockImplementation(() => {
    throw new Error('ENOENT');
  });
  const writeFileMock = jest.spyOn(fs, 'writeFileSync');

  const loadedId = loadInstallationId(filePath);

  expect(loadedId).toBe(newId);
  expect(writeFileMock).toHaveBeenCalledWith(filePath, newId);
});
test('generates a new ID if the value read from disk has an invalid format', () => {
  const newId = '0026d6e2-d9dd-409f-a972-f8ec056a636c';
  const filePath = '/test_path';
  jest.spyOn(crypto, 'randomUUID').mockReturnValue(newId);
  jest.spyOn(fs, 'readFileSync').mockImplementation(() => 'invalid_id_format');
  const writeFileMock = jest.spyOn(fs, 'writeFileSync');

  const loadedId = loadInstallationId(filePath);

  expect(loadedId).toBe(newId);
  expect(writeFileMock).toHaveBeenCalledWith(filePath, newId);
});
