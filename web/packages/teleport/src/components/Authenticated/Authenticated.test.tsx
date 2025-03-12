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
    jest.spyOn(api, 'get').mockResolvedValue(null);
    jest.spyOn(api, 'delete').mockResolvedValue(null);
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
