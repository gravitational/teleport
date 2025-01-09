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

import { createMemoryHistory } from 'history';
import { Route, Router } from 'react-router';

import { render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import { HeadlessRequest } from 'teleport/HeadlessRequest/HeadlessRequest';
import auth from 'teleport/services/auth';

test('ip address should be visible', async () => {
  jest.spyOn(auth, 'headlessSSOGet').mockImplementation(
    () =>
      new Promise(resolve => {
        resolve({ clientIpAddress: '1.2.3.4' });
      })
  );

  const headlessSSOPath = '/web/headless/2a8dcaae-1fa5-533b-aad8-f97420df44de';
  const mockHistory = createMemoryHistory({
    initialEntries: [headlessSSOPath],
  });

  render(
    <Router history={mockHistory}>
      <Route path={cfg.routes.headlessSso}>
        <HeadlessRequest />
      </Route>
    </Router>
  );

  await expect(
    screen.findByText(/Someone has initiated a command from 1.2.3.4/i)
  ).resolves.toBeInTheDocument();
});
