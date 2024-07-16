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

const testCases: { name: string; path: string; expectedPath: string }[] = [
  {
    name: 'no state and no path',
    path: '?path=',
    expectedPath: 'x-teleport-auth',
  },
  {
    name: 'no state with path',
    path: '?path=%2Ffoo%2Fbar',
    expectedPath: 'x-teleport-auth?path=%2Ffoo%2Fbar',
  },
  {
    name: 'no state with other path params (clusterId, publicAddr, publicArn',
    path: '/some-cluster-id/some-public-addr/arn::123/name',
    expectedPath:
      'x-teleport-auth?cluster=some-cluster-id&addr=some-public-addr&arn=arn%3A%3A123',
  },
  {
    name: 'no state with path and with other path params',
    path: '/some-cluster-id/some-public-addr/arn::123/name?path=%2Ffoo%2Fbar',
    expectedPath:
      'x-teleport-auth?path=%2Ffoo%2Fbar&cluster=some-cluster-id&addr=some-public-addr&arn=arn%3A%3A123',
  },
  {
    name: 'with state',
    path: '?state=ABC',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value#value=cookie-value',
  },
  {
    name: 'with state and path',
    path: '?state=ABC&path=%2Ffoo%2Fbar',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&path=%2Ffoo%2Fbar#value=cookie-value',
  },
  {
    name: 'with state, path, and params',
    path: '?state=ABC&path=%2Ffoo%2Fbar',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&path=%2Ffoo%2Fbar#value=cookie-value',
  },
];

describe('app launcher path is properly formed', () => {
  const realLocation = window.location;
  const assignMock = jest.fn();

  beforeEach(() => {
    global.fetch = jest.fn(() => Promise.resolve({})) as jest.Mock;
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(api, 'post').mockResolvedValue({});
    jest.spyOn(service, 'getAppFqdn').mockResolvedValue({
      fqdn: 'grafana.localhost',
    });
    jest.spyOn(service, 'createAppSession').mockResolvedValue({
      cookieValue: 'cookie-value',
      subjectCookieValue: 'subject-cookie-value',
      fqdn: '',
    });

    delete window.location;
    window.location = { ...realLocation, replace: assignMock };
  });

  afterEach(() => {
    window.location = realLocation;
    assignMock.mockClear();
  });

  test.each(testCases)('$name', async ({ path: query, expectedPath }) => {
    const launcherPath = `/web/launch/grafana.localhost${query}`;
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
        `https://grafana.localhost/${expectedPath}`
      )
    );
  });

  test('arn is url decoded', async () => {
    jest.spyOn(service, 'getAppFqdn').mockResolvedValue({
      fqdn: 'test-app.test.teleport',
    });
    jest.spyOn(service, 'createAppSession');

    const launcherPath =
      '/web/launch/test-app.test.teleport/test.teleport/test-app.test.teleport/arn:aws:iam::joe123:role%2FEC2FullAccess?state=ABC';
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

    await waitFor(() => {
      expect(service.createAppSession).toHaveBeenCalledWith({
        fqdn: 'test-app.test.teleport',
        clusterId: 'test.teleport',
        publicAddr: 'test-app.test.teleport',
        arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
      });
    });
  });
});
