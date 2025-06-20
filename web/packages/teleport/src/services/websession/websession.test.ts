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

import history from 'teleport/services/history';

import websession from '.';
import api from '../api';

test('user should be redirected to login even if session delete call fails', async () => {
  const mockPromise = {
    then: jest.fn().mockReturnThis(),
    catch: jest.fn().mockReturnThis(),
    finally: jest.fn().mockImplementation(callback => {
      callback();
      return mockPromise;
    }),
    [Symbol.toStringTag]: 'Promise',
  };

  jest.spyOn(api, 'delete').mockReturnValue(mockPromise as any);
  const goToLoginSpy = jest.spyOn(history, 'goToLogin').mockImplementation();
  jest.spyOn(websession, 'clear').mockImplementation();

  websession.logout();

  expect(goToLoginSpy).toHaveBeenCalled();
});
