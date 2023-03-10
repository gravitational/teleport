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

import { render, screen, waitFor } from 'design/utils/testing';
import React from 'react';
import { HeadlessSSO } from 'teleport/HeadlessSSO/HeadlessSSO';
import cfg from 'teleport/config';
import { Route, Router } from 'react-router';
import { createMemoryHistory } from 'history';
import auth from 'teleport/services/auth';

test('default error message', async () => {
  jest.spyOn(auth, 'headlessSSOGet').mockImplementation(
    () =>
      new Promise(() => {
        return { clientIpAddress: '1.1.1.1' };
      })
  );

  const headlessSSOPath = '/web/headless/00-request-id/accept';
  const mockHistory = createMemoryHistory({
    initialEntries: [headlessSSOPath],
  });

  const { container } = render(
    <Router history={mockHistory}>
      <Route path={cfg.routes.headlessSSO}>
        <HeadlessSSO />
      </Route>
    </Router>
  );

  await waitFor(() => {
    expect(
      screen.getByText(/Someone has initiated a command from/i)
    ).toBeInTheDocument();
  });
  expect(container).toMatchSnapshot();
});
