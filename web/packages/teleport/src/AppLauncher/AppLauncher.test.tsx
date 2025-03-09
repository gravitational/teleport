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
import { Router } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
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
    jest.spyOn(service, 'getAppDetails').mockResolvedValue({
      fqdn: 'grafana.localhost',
    });
    jest.spyOn(service, 'createAppSession').mockResolvedValue({
      cookieValue: 'cookie-value',
      subjectCookieValue: 'subject-cookie-value',
      fqdn: '',
    });

    delete window.location;
    window.location = {
      ...realLocation,
      replace: assignMock,
    } as unknown as string & Location;
  });

  afterEach(() => {
    window.location = {
      ...realLocation,
      replace: assignMock,
    } as unknown as string & Location;
    assignMock.mockClear();
  });

  test.each(launcherPathTestCases)(
    '$name',
    async ({ path: query, expectedPath }) => {
      render(
        <Router history={createMockHistory(`grafana.localhost${query}`)}>
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
      expect(screen.queryByText(/access denied/i)).not.toBeInTheDocument();
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

describe('fqdn is matched', () => {
  const realLocation = window.location;
  const assignMock = jest.fn();

  beforeEach(() => {
    global.fetch = jest.fn(() => Promise.resolve({})) as jest.Mock;
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(api, 'post').mockResolvedValue({});

    delete window.location;
    window.location = {
      ...realLocation,
      replace: assignMock,
    } as unknown as string & Location;
  });

  afterEach(() => {
    window.location = {
      ...realLocation,
      replace: assignMock,
    } as unknown as string & Location;
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
      jest.spyOn(service, 'getAppDetails').mockResolvedValue({
        fqdn: returnedFqdn,
      });
      jest.spyOn(service, 'createAppSession');

      render(
        <Router history={createMockHistory(path)}>
          <Route path={cfg.routes.appLauncher}>
            <AppLauncher />
          </Route>
        </Router>
      );

      await waitFor(() => {
        expect(service.createAppSession).toHaveBeenCalledWith({
          fqdn: expectedFqdn,
          cluster_name: 'test.teleport',
          public_addr: expectedPublicAddr,
          arn: expectedArn,
        });
      });

      await waitFor(() => expect(window.location.replace).toHaveBeenCalled());
      expect(screen.queryByText(/access denied/i)).not.toBeInTheDocument();
    }
  );

  test('not matching FQDN throws error', async () => {
    jest.spyOn(service, 'getAppDetails').mockResolvedValue({
      fqdn: 'different.fqdn',
    });

    render(
      <Router
        history={createMockHistory(
          'test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC'
        )}
      >
        <Route path={cfg.routes.appLauncher}>
          <AppLauncher />
        </Route>
      </Router>
    );

    await screen.findByText(/access denied/i);
    expect(
      screen.getByText(
        /failed to match applications with FQDN "test-app.test.teleport:443"/i
      )
    ).toBeInTheDocument();
    expect(window.location.replace).not.toHaveBeenCalled();
  });

  test('invalid URL when constructing a new URL with a malformed FQDN', async () => {
    jest.spyOn(service, 'getAppDetails').mockResolvedValue({
      fqdn: 'invalid.fqdn:3080:3090',
    });

    render(
      <Router
        history={createMockHistory(
          'test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC'
        )}
      >
        <Route path={cfg.routes.appLauncher}>
          <AppLauncher />
        </Route>
      </Router>
    );

    await screen.findByText(/access denied/i);
    expect(screen.getByText(/Failed to parse URL:/i)).toBeInTheDocument();
    expect(window.location.replace).not.toHaveBeenCalled();
  });
});

function createMockHistory(path: string) {
  const launcherPath = `/web/launch/${path}`;
  return createMemoryHistory({
    initialEntries: [launcherPath],
  });
}
