/**
 * Copyright 2023 Gravitational, Inc.
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
 *
 */

import { render, screen } from 'design/utils/testing';
import React from 'react';
import { Route, Router } from 'react-router';
import { createMemoryHistory } from 'history';

import { HeadlessRequest } from 'teleport/HeadlessRequest/HeadlessRequest';
import cfg from 'teleport/config';
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
