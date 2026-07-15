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

import { MemoryRouter, Route, Routes, useParams } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import cfg, { UrlLauncherParams } from 'teleport/config';
import api from 'teleport/services/api';
import service from 'teleport/services/apps';
import { storageService } from 'teleport/services/storageService';

import { AppLauncher } from './AppLauncher';

const appLauncherRoute = `${cfg.routes.appLauncher}/*`;

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
    // The ARN is percent-encoded in the URL path so that the slash in
    // "arn::123/name" does not split it into two path segments.
    name: 'no state with other path params (clusterId, publicAddr, arn)',
    path: '/some-cluster-id/some-public-addr/arn%3A%3A123%2Fname',
    expectedPath:
      'x-teleport-auth?cluster=some-cluster-id&addr=some-public-addr&arn=arn%3A%3A123%2Fname',
  },
  {
    name: 'no state with path and with other path params',
    path: '/some-cluster-id/some-public-addr/arn%3A%3A123%2Fname?path=%2Ffoo%2Fbar',
    expectedPath:
      'x-teleport-auth?path=%2Ffoo%2Fbar&cluster=some-cluster-id&addr=some-public-addr&arn=arn%3A%3A123%2Fname',
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
  {
    // First leg: the auth-exchange URL hash carries the
    // fragment, not the query string.
    name: 'no state with root path and fragment',
    path: '?path=%2F#my-section',
    expectedPath: 'x-teleport-auth?path=%2F#my-section',
  },
  {
    name: 'no state with path and fragment',
    path: '?path=%2Ffoo%2Fbar#my-section',
    expectedPath: 'x-teleport-auth?path=%2Ffoo%2Fbar#my-section',
  },
  {
    name: 'no state with path, query, and fragment',
    path: '?path=%2Ffoo%2Fbar&query=q%3Dv#my-section',
    expectedPath: 'x-teleport-auth?path=%2Ffoo%2Fbar%3Fq%3Dv#my-section',
  },
  {
    // Second leg: fragment is repacked alongside the session
    // cookie in the URL hash for the inline JS in
    // lib/web/app/redirect.go to reattach.
    name: 'with state, path, and fragment',
    path: '?state=ABC&path=%2Ffoo%2Fbar#my-section',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&path=%2Ffoo%2Fbar#value=cookie-value&fragment=my-section',
  },
  {
    // Second leg, no path: hits the inline-JS branch that
    // navigates to the origin root with the fragment attached.
    name: 'with state and fragment, no path',
    path: '?state=ABC#my-section',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value#value=cookie-value&fragment=my-section',
  },
  {
    // OAuth-style fragment: `=` and `&` round-trip through
    // URLSearchParams encoding.
    name: 'with state, path, and OAuth implicit-flow fragment',
    path: '?state=ABC&path=%2Fcallback#access_token=secret&token_type=Bearer',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&path=%2Fcallback#value=cookie-value&fragment=access_token%3Dsecret%26token_type%3DBearer',
  },
  {
    // Chain redirect: launcher drops the fragment to avoid
    // leaking it to intermediate apps' origins.
    name: 'with state, path, fragment, and required-apps chain',
    path: '?state=ABC&path=%2Ffoo&required-apps=app1,app2#secret',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&required-apps=app1%2Capp2&path=%2Ffoo#value=cookie-value',
  },
];

describe('app launcher path is properly formed', () => {
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
  });

  const windowLocation = {
    replace: jest.fn(),
  };

  test.each(launcherPathTestCases)(
    '$name',
    async ({ path: query, expectedPath }) => {
      render(
        <MemoryRouter
          initialEntries={[`/web/launch/grafana.localhost${query}`]}
        >
          <Routes>
            <Route
              path={appLauncherRoute}
              element={<AppLauncher windowLocation={windowLocation} />}
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() =>
        expect(windowLocation.replace).toHaveBeenCalledWith(
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
    // The ARN is percent-encoded in the URL path. React Router's
    // useParams() auto-decodes the %2F back to /, so the ARN arrives
    // fully decoded for createAppSession.
    name: 'ARN URL',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport/arn%3Aaws%3Aiam%3A%3Ajoe123%3Arole%2FEC2FullAccess?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: 'arn:aws:iam::joe123:role/EC2FullAccess',
  },
  {
    name: 'ARN URL with multi-level path',
    path: 'test-app.test.teleport/test.teleport/test-app.test.teleport/arn%3Aaws%3Aiam%3A%3Ajoe123%3Arole%2Fpath%2Fto%2FEC2FullAccess?state=ABC',
    returnedFqdn: 'test-app.test.teleport',
    expectedFqdn: 'test-app.test.teleport',
    expectedPublicAddr: 'test-app.test.teleport',
    expectedArn: 'arn:aws:iam::joe123:role/path/to/EC2FullAccess',
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
  beforeEach(() => {
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(api, 'post').mockResolvedValue({});
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
      const windowLocation = {
        replace: jest.fn(),
      };

      render(
        <MemoryRouter initialEntries={[`/web/launch/${path}`]}>
          <Routes>
            <Route
              path={appLauncherRoute}
              element={<AppLauncher windowLocation={windowLocation} />}
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() => {
        expect(service.createAppSession).toHaveBeenCalledWith({
          fqdn: expectedFqdn,
          cluster_name: 'test.teleport',
          public_addr: expectedPublicAddr,
          arn: expectedArn,
        });
      });

      await waitFor(() => expect(windowLocation.replace).toHaveBeenCalled());
      expect(screen.queryByText(/access denied/i)).not.toBeInTheDocument();
    }
  );

  test('not matching FQDN throws error', async () => {
    jest.spyOn(service, 'getAppDetails').mockResolvedValue({
      fqdn: 'different.fqdn',
    });
    const windowLocation = {
      replace: jest.fn(),
    };

    render(
      <MemoryRouter
        initialEntries={[
          '/web/launch/test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC',
        ]}
      >
        <Routes>
          <Route
            path={appLauncherRoute}
            element={<AppLauncher windowLocation={windowLocation} />}
          />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText(/access denied/i);
    expect(
      screen.getByText(
        /failed to match applications with FQDN "test-app.test.teleport:443"/i
      )
    ).toBeInTheDocument();
    expect(windowLocation.replace).not.toHaveBeenCalled();
  });

  test('invalid URL when constructing a new URL with a malformed FQDN', async () => {
    jest.spyOn(service, 'getAppDetails').mockResolvedValue({
      fqdn: 'invalid.fqdn:3080:3090',
    });
    const windowLocation = {
      replace: jest.fn(),
    };

    render(
      <MemoryRouter
        initialEntries={[
          '/web/launch/test-app.test.teleport:443/test.teleport/test-app.test.teleport:443?state=ABC',
        ]}
      >
        <Routes>
          <Route
            path={appLauncherRoute}
            element={<AppLauncher windowLocation={windowLocation} />}
          />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText(/access denied/i);
    expect(screen.getByText(/Failed to parse URL:/i)).toBeInTheDocument();
    expect(windowLocation.replace).not.toHaveBeenCalled();
  });
});

// stashedFragmentTestCases cover the logged-out flow: the
// Authenticated wrapper stashes the hash in sessionStorage before
// redirecting to /web/login, and the launcher consumes it after
// the post-login navigation. The launcher cannot read the original
// hash from `useLocation()` at this point because goToLogin
// dropped it from `redirect_uri` and the JS-driven navigation back
// to the launcher does not inherit fragments.
const stashedFragmentTestCases: {
  name: string;
  path: string;
  stashedPath: string;
  stashedHash: string;
  expectedPath: string;
}[] = [
  {
    // Stashed fragment threads through the first leg.
    name: 'no state with stashed fragment',
    path: '?path=%2Ffoo%2Fbar',
    stashedPath: '/web/launch/grafana.localhost',
    stashedHash: '#my-section',
    expectedPath: 'x-teleport-auth?path=%2Ffoo%2Fbar#my-section',
  },
  {
    // Stashed fragment threads through the second leg, packed
    // alongside the session cookie.
    name: 'with state and stashed fragment',
    path: '?state=ABC&path=%2Ffoo%2Fbar',
    stashedPath: '/web/launch/grafana.localhost',
    stashedHash: '#my-section',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&path=%2Ffoo%2Fbar#value=cookie-value&fragment=my-section',
  },
  {
    // Stash for a different launcher path is ignored.
    name: 'stash for a different path is ignored',
    path: '?path=%2Ffoo%2Fbar',
    stashedPath: '/web/launch/other.localhost',
    stashedHash: '#my-section',
    expectedPath: 'x-teleport-auth?path=%2Ffoo%2Fbar',
  },
  {
    // Required-apps chain drops the stashed fragment, same as for
    // a URL fragment on the logged-in path.
    name: 'required-apps chain drops stashed fragment',
    path: '?state=ABC&path=%2Ffoo&required-apps=app1,app2',
    stashedPath: '/web/launch/grafana.localhost',
    stashedHash: '#secret',
    expectedPath:
      'x-teleport-auth?state=ABC&subject=subject-cookie-value&required-apps=app1%2Capp2&path=%2Ffoo#value=cookie-value',
  },
];

describe('app launcher consumes a stashed fragment when the URL has none', () => {
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
    window.sessionStorage.clear();
  });

  afterEach(() => {
    window.sessionStorage.clear();
  });

  test.each(stashedFragmentTestCases)(
    '$name',
    async ({ path: query, stashedPath, stashedHash, expectedPath }) => {
      storageService.setAppLauncherFragment(stashedPath, stashedHash);

      const windowLocation = {
        replace: jest.fn(),
      };
      render(
        <MemoryRouter
          initialEntries={[`/web/launch/grafana.localhost${query}`]}
        >
          <Routes>
            <Route
              path={appLauncherRoute}
              element={<AppLauncher windowLocation={windowLocation} />}
            />
          </Routes>
        </MemoryRouter>
      );

      await waitFor(() =>
        expect(windowLocation.replace).toHaveBeenCalledWith(
          `https://grafana.localhost/${expectedPath}`
        )
      );
      expect(screen.queryByText(/access denied/i)).not.toBeInTheDocument();
    }
  );

  test('the URL fragment wins when both are present', async () => {
    storageService.setAppLauncherFragment(
      '/web/launch/grafana.localhost',
      '#stashed'
    );

    const windowLocation = {
      replace: jest.fn(),
    };
    render(
      <MemoryRouter
        initialEntries={[`/web/launch/grafana.localhost?path=%2Ffoo#fromurl`]}
      >
        <Routes>
          <Route
            path={appLauncherRoute}
            element={<AppLauncher windowLocation={windowLocation} />}
          />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() =>
      expect(windowLocation.replace).toHaveBeenCalledWith(
        `https://grafana.localhost/x-teleport-auth?path=%2Ffoo#fromurl`
      )
    );
  });

  test('the stash is cleared after the launcher reads it', async () => {
    storageService.setAppLauncherFragment(
      '/web/launch/grafana.localhost',
      '#my-section'
    );

    const windowLocation = {
      replace: jest.fn(),
    };
    render(
      <MemoryRouter
        initialEntries={[`/web/launch/grafana.localhost?path=%2Ffoo`]}
      >
        <Routes>
          <Route
            path={appLauncherRoute}
            element={<AppLauncher windowLocation={windowLocation} />}
          />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => expect(windowLocation.replace).toHaveBeenCalled());
    expect(
      storageService.consumeAppLauncherFragment('/web/launch/grafana.localhost')
    ).toBe('');
  });
});

// Round-trip test: verifies that an ARN with slashes survives the full
// cycle from URL generation through React Router matching and param
// decoding.
describe('ARN round-trips through URL generation and routing', () => {
  function ArnCapture({ onArn }: { onArn: (arn: string) => void }) {
    const params = useParams<UrlLauncherParams>();
    const arn = params.arn ? decodeURIComponent(params.arn) : undefined;
    if (arn) {
      onArn(arn);
    }
    return <div>captured</div>;
  }

  test.each([
    'arn:aws:iam::123456789012:role/my-role',
    'arn:aws:iam::123456789012:role/path/to/my-role',
    'arn:aws:iam::123456789012:role/path+with=chars',
  ])('round-trips ARN: %s', async rawArn => {
    const url = cfg.getAppLauncherRoute({
      fqdn: 'app.example.com',
      clusterId: 'cluster1',
      publicAddr: 'app.example.com',
      arn: rawArn,
    });

    let capturedArn: string | undefined;
    render(
      <MemoryRouter initialEntries={[url]}>
        <Routes>
          <Route
            path={`${cfg.routes.appLauncher}/*`}
            element={<ArnCapture onArn={arn => (capturedArn = arn)} />}
          />
        </Routes>
      </MemoryRouter>
    );

    await screen.findByText('captured');
    expect(capturedArn).toBe(rawArn);
  });
});
