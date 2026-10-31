/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { ApiError } from '../api/parseError';
import {
  isPathNotFoundError,
  withGenericUnsupportedError,
} from './unsupported';

afterEach(() => {
  jest.resetAllMocks();
});

describe('withGenericUnsupportedError', () => {
  test('path not found error with proxy version throws custom error', async () => {
    const pathNotFoundError = new ApiError({
      message: '',
      response: { status: 404 } as Response,
      proxyVersion: {
        major: 1,
        minor: 2,
        patch: 3,
        preRelease: 'dev',
        string: '1.2.3-dev',
      },
    });

    expect(() =>
      withGenericUnsupportedError(pathNotFoundError, '2.0.0')
    ).toThrow('Your proxy (v1.2.3-dev) may be behind');

    expect(() =>
      withGenericUnsupportedError(pathNotFoundError, '2.0.0')
    ).toThrow('minimum required version (v2.0.0)');
  });

  test('non path related 404 error rethrows same error', async () => {
    const resourceNotFoundError = new ApiError({
      message: `same error`,
      response: { status: 404 } as Response,
    });

    expect(() =>
      withGenericUnsupportedError(resourceNotFoundError, 'v2.0.0')
    ).toThrow('same error');
  });
});

describe('isPathNotFoundError', () => {
  test('error resulting from path not found', () => {
    const pathNotFoundError = new ApiError({
      message: 'some error',
      response: { status: 404 } as Response,
      proxyVersion: {
        major: 1,
        minor: 2,
        patch: 3,
        preRelease: 'dev',
        string: 'v1.2.3-dev',
      },
    });

    expect(isPathNotFoundError(pathNotFoundError)).toBe(true);
  });

  test('error unrelated to path not found', () => {
    const resourceNotFoundError = new ApiError({
      message: `some error`,
      response: { status: 404 } as Response,
    });

    expect(isPathNotFoundError(resourceNotFoundError)).toBe(false);
  });
});
