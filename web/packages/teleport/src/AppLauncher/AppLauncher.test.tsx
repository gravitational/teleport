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

import React from 'react';
import { render, waitFor } from 'design/utils/testing';
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { Route } from 'teleport/components/Router';
import api from 'teleport/services/api';
import cfg from 'teleport/config';
import service from 'teleport/services/apps';

import { AppLauncher } from './AppLauncher';

const testCases: { name: string; query: string; expectedPath: string }[] = [
  {
    name: 'no path or query',
    query: '?path=',
    expectedPath: '',
  },
  {
    name: 'root path',
    query: '?path=%2F',
    expectedPath: '/',
  },
  {
    name: 'with multi path',
    query: '?path=%2Ffoo%2Fbar',
    expectedPath: '/foo/bar',
  },
  {
    name: 'with only query',
    query: '?path=&query=foo%3Dbar',
    expectedPath: '?foo=bar',
  },
  {
    name: 'with query with same keys used to store the original path and query',
    query: '?path=foo&query=foo%3Dbar%26query%3Dtest1%26path%3Dtest',
    expectedPath: '/foo?foo=bar&query=test1&path=test',
  },
  {
    name: 'with query and root path',
    query: '?path=%2F&query=foo%3Dbar%26baz%3Dqux%26fruit%3Dapple',
    expectedPath: '/?foo=bar&baz=qux&fruit=apple',
  },
  {
    name: 'queries with encoded spaces',
    query:
      '?path=%2Falerting%2Flist&query=search%3Dstate%3Ainactive%2520type%3Aalerting%2520health%3Anodata',
    expectedPath:
      '/alerting/list?search=state:inactive%20type:alerting%20health:nodata',
  },
  {
    name: 'queries with non-encoded spaces',
    query:
      '?path=%2Falerting+%2Flist&query=search%3Dstate%3Ainactive+type%3Aalerting+health%3Anodata',
    expectedPath:
      '/alerting /list?search=state:inactive type:alerting health:nodata',
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

  test.each(testCases)('$name', async ({ query, expectedPath }) => {
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
        `https://grafana.localhost${expectedPath}`
      )
    );
  });

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
});
