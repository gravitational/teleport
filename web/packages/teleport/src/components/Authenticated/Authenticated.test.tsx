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

import { render, screen, waitFor } from 'design/utils/testing';

import session from 'teleport/services/websession';
import { ApiError } from 'teleport/services/api/parseError';
import api from 'teleport/services/api';
import history from 'teleport/services/history';

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
    jest.spyOn(session, 'validateCookieAndSession').mockResolvedValue(null);
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
    const mockForbiddenError = new ApiError('some error', {
      status: 403,
    } as Response);

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
