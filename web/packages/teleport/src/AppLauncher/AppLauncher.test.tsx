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
 */

import React from 'react';
import { render } from 'design/utils/testing';

import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { Route } from 'teleport/components/Router';

import cfg from 'teleport/config';

import service from 'teleport/services/apps';

import { AppLauncher } from './AppLauncher';

test('arn is url decoded', () => {
  jest.spyOn(service, 'createAppSession');

  const launcherPath =
    '/web/launch/test-app.test.teleport/test.teleport/test-app.test.teleport/arn:aws:iam::joe123:role%2FEC2FullAccess';
  const mockHistory = createMemoryHistory({
    initialEntries: [launcherPath],
  });

  render(
    <Router history={mockHistory}>
      <Route path={cfg.routes.appLauncher}>
        <AppLauncher />
      </Route>
    </Router>
  );

  expect(service.createAppSession).toHaveBeenCalledWith({
    fqdn: 'test-app.test.teleport',
    clusterId: 'test.teleport',
    publicAddr: 'test-app.test.teleport',
    arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
  });
});
