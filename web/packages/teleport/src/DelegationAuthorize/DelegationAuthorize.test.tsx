/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { MemoryRouter } from 'react-router';

import { render, screen, waitFor } from 'design/utils/testing';

import { Switch } from 'teleport/components/Router';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
import history from 'teleport/services/history';
import userService from 'teleport/services/user';
import session from 'teleport/services/websession';
import { getDelegationAuthorizeRoute } from 'teleport/Teleport';

jest.mock('shared/libs/logger', () => {
  const mockLogger = {
    error: jest.fn(),
    warn: jest.fn(),
  };

  return {
    create: () => mockLogger,
  };
});

function renderDelegationAuthorizeRoute() {
  render(
    <MemoryRouter initialEntries={[cfg.routes.delegationAuthorize]}>
      <Switch>{getDelegationAuthorizeRoute()}</Switch>
    </MemoryRouter>
  );
}

describe('DelegationAuthorize', () => {
  beforeEach(() => {
    jest.spyOn(session, 'isValid').mockReturnValue(true);
    jest.spyOn(session, 'validateCookieAndSession').mockResolvedValue({});
    jest.spyOn(session, 'ensureSession').mockImplementation();
    jest.spyOn(session, 'getInactivityTimeout').mockReturnValue(0);
    jest.spyOn(session, 'clear').mockImplementation();
    jest.spyOn(session, 'clearBrowserSession').mockImplementation();
    jest.spyOn(api, 'get').mockResolvedValue({});
    jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(null);
    jest.spyOn(history, 'goToLogin').mockImplementation();
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  test('renders at the delegation authorization route for authenticated users', async () => {
    renderDelegationAuthorizeRoute();

    expect(await screen.findByText('Hello World')).toBeInTheDocument();
    expect(session.validateCookieAndSession).toHaveBeenCalledTimes(1);
  });

  test('redirects unauthenticated users to login', async () => {
    jest.spyOn(session, 'isValid').mockReturnValue(false);

    renderDelegationAuthorizeRoute();

    await waitFor(() =>
      expect(session.clearBrowserSession).toHaveBeenCalledWith(true)
    );

    expect(screen.queryByText('Hello World')).not.toBeInTheDocument();
  });
});
