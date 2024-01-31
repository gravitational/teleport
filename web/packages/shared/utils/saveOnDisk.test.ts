/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { saveOnDisk } from './saveOnDisk';

test('saveOnDisk', async () => {
  const element = { href: jest.fn(), click: jest.fn(), download: jest.fn() };
  const createElement = jest
    .spyOn(document, 'createElement')
    .mockReturnValueOnce(element as any);

  jest.spyOn(document.body, 'appendChild').mockImplementation();
  jest.spyOn(document.body, 'removeChild').mockImplementation();
  const blob = jest.spyOn(global, 'Blob').mockImplementation();
  // eslint-disable-next-line jest/prefer-spy-on
  window.URL.createObjectURL = jest.fn();

  saveOnDisk('testcontent', 'testfile.txt', 'plain/text');
  expect(createElement).toHaveBeenCalledWith('a');
  expect(element.download).toBe('testfile.txt');
  expect(blob).toHaveBeenCalledWith(['testcontent'], { type: 'plain/text' });
});
