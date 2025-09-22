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

import { waitFor } from '@testing-library/react';

import history from 'teleport/services/history';

import websession from '.';
import api from '../api';

test('user should be redirected to login even if session delete call fails', async () => {
  jest.spyOn(console, 'error').mockImplementation();
  jest.spyOn(api, 'delete').mockRejectedValue(new Error('some error'));
  const goToLoginSpy = jest.spyOn(history, 'goToLogin').mockImplementation();
  jest.spyOn(websession, 'clear').mockImplementation();

  websession.logout();

  await waitFor(() => expect(goToLoginSpy).toHaveBeenCalled());
});
