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

import { render, screen, waitFor } from 'design/utils/testing';

import api from 'teleport/services/api';
import { ApiError } from 'teleport/services/api/parseError';
import history from 'teleport/services/history';
import { storageService } from 'teleport/services/storageService';
import userService from 'teleport/services/user';
import session from 'teleport/services/websession';

import Authenticated from './Authenticated';

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

describe('session', () => {
  beforeEach(() => {
    jest.spyOn(session, 'isValid').mockImplementation(() => true);
    jest.spyOn(session, 'validateCookieAndSession').mockResolvedValue({});
    jest.spyOn(session, 'ensureSession').mockImplementation();
    jest.spyOn(session, 'getInactivityTimeout').mockImplementation(() => 0);
    jest.spyOn(session, 'clear').mockImplementation();
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(api, 'delete').mockResolvedValue(null);
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(null);
    jest.spyOn(history, 'goToLogin').mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('valid session and valid cookie', async () => {
    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    const targetEl = await screen.findByText(/hello world/i);

    expect(targetEl).toBeInTheDocument();
    expect(session.isValid).toHaveBeenCalledTimes(1);
    expect(session.validateCookieAndSession).toHaveBeenCalledTimes(1);
    expect(session.ensureSession).toHaveBeenCalledTimes(1);
    expect(history.goToLogin).not.toHaveBeenCalled();
  });

  test('valid session and invalid cookie', async () => {
    const mockForbiddenError = new ApiError({
      message: 'some error',
      response: {
        status: 403,
      } as Response,
    });

    jest
      .spyOn(session, 'validateCookieAndSession')
      .mockRejectedValue(mockForbiddenError);

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() => expect(history.goToLogin).toHaveBeenCalledTimes(1));
    expect(session.clear).toHaveBeenCalledTimes(1);

    expect(screen.queryByText(/hello world/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/go to login/i)).not.toBeInTheDocument();
    expect(session.ensureSession).not.toHaveBeenCalled();
  });

  test('invalid session', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => false);

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() => expect(session.clear).toHaveBeenCalledTimes(1));
    expect(history.goToLogin).toHaveBeenCalledTimes(1);

    expect(screen.queryByText(/hello world/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/go to login/i)).not.toBeInTheDocument();
    expect(session.validateCookieAndSession).not.toHaveBeenCalled();
    expect(session.ensureSession).not.toHaveBeenCalled();
  });

  test('non-authenticated related error', async () => {
    jest
      .spyOn(session, 'validateCookieAndSession')
      .mockRejectedValue(new Error('some network error'));

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    const targetEl = await screen.findByText('some network error');
    expect(targetEl).toBeInTheDocument();

    expect(screen.queryByText(/hello world/i)).not.toBeInTheDocument();
    expect(session.ensureSession).not.toHaveBeenCalled();
    expect(history.goToLogin).not.toHaveBeenCalled();
  });
});

describe('app launcher fragment stash', () => {
  function setLocation({ pathname, search = '', hash = '' }) {
    window.history.replaceState({}, '', `${pathname}${search}${hash}`);
  }

  beforeEach(() => {
    jest.spyOn(session, 'validateCookieAndSession').mockResolvedValue({});
    jest.spyOn(session, 'ensureSession').mockImplementation();
    jest.spyOn(session, 'getInactivityTimeout').mockImplementation(() => 0);
    jest.spyOn(session, 'clearBrowserSession').mockImplementation();
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(null);
    jest.spyOn(history, 'goToLogin').mockImplementation();
    jest.spyOn(storageService, 'setAppLauncherFragment');
  });

  afterEach(() => {
    window.history.replaceState({}, '', '/');
    jest.clearAllMocks();
  });

  test('stashes the fragment when invalid session on launcher route', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => false);
    setLocation({
      pathname: '/web/launch/grafana.localhost',
      search: '?path=%2Ffoo',
      hash: '#my-section',
    });

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() =>
      expect(storageService.setAppLauncherFragment).toHaveBeenCalledWith(
        '/web/launch/grafana.localhost',
        '#my-section'
      )
    );
  });

  test('stashes the fragment when 403 on launcher route', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => true);
    jest.spyOn(session, 'validateCookieAndSession').mockRejectedValue(
      new ApiError({
        message: 'forbidden',
        response: { status: 403 } as Response,
      })
    );
    setLocation({
      pathname: '/web/launch/grafana.localhost',
      hash: '#my-section',
    });

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() =>
      expect(storageService.setAppLauncherFragment).toHaveBeenCalledWith(
        '/web/launch/grafana.localhost',
        '#my-section'
      )
    );
  });

  test('does not stash on a non-launcher route', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => false);
    setLocation({
      pathname: '/web/cluster/test/resources',
      hash: '#some-anchor',
    });

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() => expect(session.clearBrowserSession).toHaveBeenCalled());
    expect(storageService.setAppLauncherFragment).not.toHaveBeenCalled();
  });

  test('does not stash when no hash is present', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => false);
    setLocation({
      pathname: '/web/launch/grafana.localhost',
      search: '?path=%2Ffoo',
    });

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() => expect(session.clearBrowserSession).toHaveBeenCalled());
    expect(storageService.setAppLauncherFragment).not.toHaveBeenCalled();
  });

  test('does not stash on a required-apps chain', async () => {
    jest.spyOn(session, 'isValid').mockImplementation(() => false);
    setLocation({
      pathname: '/web/launch/grafana.localhost',
      search: '?required-apps=app1,app2',
      hash: '#secret',
    });

    render(
      <Authenticated>
        <div>hello world</div>
      </Authenticated>
    );

    await waitFor(() => expect(session.clearBrowserSession).toHaveBeenCalled());
    expect(storageService.setAppLauncherFragment).not.toHaveBeenCalled();
  });
});
