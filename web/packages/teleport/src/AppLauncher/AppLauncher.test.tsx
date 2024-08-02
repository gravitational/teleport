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

const launcherPathTestCases: {
  name: string;
  path: string;
  expectedPath: string;
}[] = [
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

  test.each(launcherPathTestCases)(
    '$name',
    async ({ path: query, expectedPath }) => {
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
    }
  );
});

const appSessionTestCases: {
  name: string;
  path: string;
  returnedFqdn: string;
  expectedFqdn: string;
  expectedPublicAddr: string;
  expectedArn: string;
}[] = [
  {
    name: 'ARN URL',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport/arn:aws:iam::joe123:role%2FEC2FullAccess?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: 'arn:aws:iam::joe123:role/EC2FullAccess',
  },
  {
    name: 'uppercase resolved FQDN',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'TEST-APP.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase public addr',
    path: 'test-app.test.teleport/test.teleport/TEST-APP.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'TEST-APP.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase FQDN',
    path: 'TEST-APP.test.teleport/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase resolved FQDN, public addr',
    path: 'test-app.test.teleport/test.teleport/TEST-APP.test.teleport?state=ABC',
    returnedFqdn: 'TEST-APP.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'TEST-APP.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase resolved FQDN,FQDN',
    path: 'TEST-APP.test.teleport/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'TEST-APP.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase public addr, FQDN',
    path: 'TEST-APP.test.teleport/test.teleport/TEST-APP.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'TEST-APP.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'uppercase FQDN, resolved FQDN, public addr',
    path: 'TEST-APP.test.teleport/test.teleport/TEST-APP.test.teleport?state=ABC',
    returnedFqdn: 'TEST-APP.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'TEST-APP.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'public addr with port',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport:443?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'FQDN with port',
    path: 'test-app.test.teleport:443/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport:443',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'resolved FQDN with port',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport:443',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'FQDN, public addr with port',
    path: 'test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport:443',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'FQDN, resolved FQDN with port',
    path: 'test-app.test.teleport:443/test.teleport/test-app.test.teleport?state=ABC',
    returnedFqdn: 'test-app.test.teleport:443',
    expectedFqdn: 'test-app.test.teleport:443',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'public addr, resolved FQDN with port',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport:443?state=ABC',
    returnedFqdn: 'test-app.test.teleport:443',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
  {
    name: 'FQDN, public addr, resolved FQDN with port',
    path: 'test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC',
    returnedFqdn: 'test-app.test.teleport:443',
    expectedFqdn: 'test-app.test.teleport:443',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: undefined,
  },
];

describe('app session request is properly formed', () => {
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

  test.each(appSessionTestCases)(
    '$name',
    async ({
      path,
      returnedFqdn,
      expectedFqdn,
      expectedPublicAddr,
      expectedArn,
    }) => {
      jest.spyOn(service, 'getAppFqdn').mockResolvedValue({
        fqdn: returnedFqdn,
      });
      jest.spyOn(service, 'createAppSession');

      const launcherPath = `/web/launch/${path}`;
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
          fqdn: expectedFqdn,
          clusterId: 'test.teleport',
          publicAddr: expectedPublicAddr,
          arn: expectedArn,
        });
      });
    }
  );
});
