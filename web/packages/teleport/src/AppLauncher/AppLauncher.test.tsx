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
import { render, waitFor } from 'design/utils/testing';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { Route } from 'teleport/components/Router';
import api from 'teleport/services/api';
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

const testCases = [
  {
    queryParams: '?path=%2F',
    expectedPath: '/',
  },
  {
    queryParams: '?path=%2Flogin',
    expectedPath: '/login',
  },
  {
    queryParams: '?path=%2Ffoo%2Fbar&fruit=apple&os=mac',
    expectedPath: '/foo/bar?fruit=apple&os=mac',
  },
  {
    queryParams: '?path=',
    expectedPath: '/',
  },
  {
    queryParams: '?path=&fruit=apple',
    expectedPath: '/?fruit=apple',
  },
  {
    queryParams:
      '?path=%2Falerting%2Flist&search=state:pending%20type:recording%20health:error',
    expectedPath:
      '/alerting/list?search=state:pending+type:recording+health:error',
  },
];

describe('app launcher path is properly formed', () => {
  const realLocation = window.location;
  const assignMock = jest.fn();

  beforeEach(() => {
    global.fetch = jest.fn(() => Promise.resolve({})) as jest.Mock;
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(api, 'post').mockResolvedValue({});

    delete window.location;
    window.location = { ...realLocation, replace: assignMock };
  });

  afterEach(() => {
    window.location = realLocation;
    assignMock.mockClear();
  });

  test.each(testCases)(
    '$queryParams',
    async ({ queryParams, expectedPath }) => {
      const launcherPath = `/web/launch/grafana.localhost${queryParams}`;
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

      await waitFor(() =>
        expect(window.location.replace).toHaveBeenCalledWith(
          `https://grafana.localhost${expectedPath}`
        )
      );
    }
  );
});
