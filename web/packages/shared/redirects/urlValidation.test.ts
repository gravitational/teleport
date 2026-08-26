/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { validateClientRedirect } from './urlValidation';

describe('validateClientRedirect', () => {
  test.each([
    'http://localhost:12345?response=abc',
    'https://localhost:12345?response=abc',
    'http://127.0.0.1:12345?response=abc',
    'http://[::1]:12345?response=abc',
  ])('accepts loopback URL: %s', url => {
    expect(() => validateClientRedirect(url)).not.toThrow();
  });

  test('rejects empty URL', () => {
    expect(() => validateClientRedirect('')).toThrow(
      'redirect URL must not be empty'
    );
  });

  test('rejects non-local address', () => {
    expect(() =>
      validateClientRedirect('http://example.com:12345?response=abc')
    ).toThrow('example.com is not a valid local address');
  });

  test('rejects unsupported protocol', () => {
    expect(() =>
      validateClientRedirect('ftp://localhost:12345?response=abc')
    ).toThrow('ftp: is not a valid protocol');
  });

  test('rejects URL with credentials', () => {
    expect(() =>
      validateClientRedirect('http://user:pass@localhost:12345?response=abc')
    ).toThrow('redirect URL must not contain credentials');
  });
});
