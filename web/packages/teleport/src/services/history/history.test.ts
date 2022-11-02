/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { createMemoryHistory } from 'history';

import history from './history';

describe('services/history', () => {
  const fallbackRoute = '/web';
  let browserHistory;

  beforeEach(() => {
    history.init(createMemoryHistory());
    browserHistory = history.original(/* be default returns inMemory history*/);
    jest.spyOn(browserHistory, 'push');
    jest.spyOn(history, 'getRoutes');
    jest.spyOn(history, '_pageRefresh').mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe('ensureBaseUrl', () => {
    it('should always ensure the base url matched cfg.baseUrl', () => {
      expect(history.ensureBaseUrl('')).toBe('http://localhost/');
      expect(history.ensureBaseUrl('/')).toBe('http://localhost/');
      expect(history.ensureBaseUrl('/web')).toBe('http://localhost/web');
      expect(history.ensureBaseUrl('somepath')).toBe(
        'http://localhost/somepath'
      );
      expect(history.ensureBaseUrl('http://badurl')).toBe('http://localhost/');
      // app access path redirects
      expect(
        history.ensureBaseUrl(
          '/web/launch%3Fpath%3D%252Fteleport%252Fconfig%252F'
        )
      ).toBe(
        'http://localhost/web/launch%3Fpath%3D%252Fteleport%252Fconfig%252F'
      );
      expect(
        history.ensureBaseUrl(
          'http://badurl/web/launch%3Fpath%3D%252Fteleport%252Fconfig%252F'
        )
      ).toBe(
        'http://localhost/web/launch%3Fpath%3D%252Fteleport%252Fconfig%252F'
      );
    });
  });

  describe('canPush', () => {
    const push = actual => ({
      andExpect(expected) {
        history.push(actual);
        expect(browserHistory.push).toHaveBeenCalledWith(expected);
      },
    });

    // eslint-disable-next-line jest/expect-expect
    it('should push if allowed else fallback to default route', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue([
          '/valid',
          '/',
          '/test/:param',
          '/test/:param/:optional?',
          '/web/cluster/:siteId/node/:serverId/:login/:sid?',
        ]);
      push('invalid').andExpect(fallbackRoute);
      push('.').andExpect(fallbackRoute);
      push('/valid/test').andExpect(fallbackRoute);
      push('@#4').andExpect(fallbackRoute);
      push('/valid').andExpect('/valid');
      push('').andExpect('');
      push('/').andExpect('/');
      push('/test/param1').andExpect('/test/param1');
      push('/test/param1/param2').andExpect('/test/param1/param2');

      // test option parameters
      push('/web/cluster/one/node/xxx/root/yyyy/xxx/unknown').andExpect(
        fallbackRoute
      );
      push('/web/cluster/one/node/xxx/root').andExpect(
        '/web/cluster/one/node/xxx/root'
      );
      push('/web/cluster/one/node/xxx/root/yyy').andExpect(
        '/web/cluster/one/node/xxx/root/yyy'
      );
    });

    it('should refresh a page if called withRefresh=true', () => {
      let route = '/';
      jest.spyOn(history, 'getRoutes').mockReturnValue([route]);
      history.push(route, true);
      expect(history._pageRefresh).toHaveBeenCalledWith(route);
    });
  });

  describe('goToLogin()', () => {
    it('should navigate to login with URL that has redirect parameter with current location', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue(['/web/login', '/current-location']);
      history.original().location.pathname = '/current-location';
      history.goToLogin(true);

      const expected =
        '/web/login?redirect_uri=http://localhost/current-location';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });
  });
});
