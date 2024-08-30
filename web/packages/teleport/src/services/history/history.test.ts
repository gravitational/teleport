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
      history.goToLogin({ rememberLocation: true });

      const expected =
        '/web/login?redirect_uri=http://localhost/current-location';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });

    it('should navigate to login with access_changed param and no redirect_uri', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue(['/web/login', '/current-location']);
      history.original().location.pathname = '/current-location';
      history.goToLogin({ withAccessChangedMessage: true });

      const expected = '/web/login?access_changed';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });

    it('should navigate to login with access_changed param and redirect_uri', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue(['/web/login', '/current-location']);
      history.original().location.pathname = '/current-location';
      history.goToLogin({
        rememberLocation: true,
        withAccessChangedMessage: true,
      });

      const expected =
        '/web/login?access_changed&redirect_uri=http://localhost/current-location';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });

    it('should navigate to login with no params', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue(['/web/login', '/current-location']);
      history.original().location.pathname = '/current-location';
      history.goToLogin();

      const expected = '/web/login';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });

    it('should preserve query params in the redirect_uri', () => {
      jest
        .spyOn(history, 'getRoutes')
        .mockReturnValue(['/web/login', '/current-location']);
      history.original().location.pathname = '/current-location?test=value';
      history.goToLogin({
        rememberLocation: true,
        withAccessChangedMessage: true,
      });

      const expected =
        '/web/login?access_changed&redirect_uri=http://localhost/current-location?test=value';
      expect(history._pageRefresh).toHaveBeenCalledWith(expected);
    });
  });
});
