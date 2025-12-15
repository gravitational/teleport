/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { renderHook } from '@testing-library/react';

import cfg from 'teleport/config';
import history from 'teleport/services/history';
import session from 'teleport/services/websession';

import useLogin from './useLogin';

beforeEach(() => {
  jest.restoreAllMocks();
  jest.spyOn(session, 'isValid').mockImplementation(() => true);
  jest.spyOn(history, 'push').mockImplementation();
  jest.spyOn(history, 'replace').mockImplementation();
  jest.mock('shared/hooks', () => ({
    useAttempt: () => {
      return [
        { status: 'success', statusText: 'Success Text' },
        {
          clear: jest.fn(),
        },
      ];
    },
  }));
});

afterEach(() => {
  jest.resetAllMocks();
});

it('redirect to root on path not matching "/enterprise/saml-idp/sso"', () => {
  jest.spyOn(history, 'getRedirectParam').mockReturnValue('http://localhost');
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');

  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue('http://localhost/web/cluster/name/resources');
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');
});

it('redirect to SAML SSO path on matching "/enterprise/saml-idp/sso"', () => {
  const samlIdpPath = new URL('http://localhost' + cfg.routes.samlIdpSso);
  cfg.baseUrl = 'http://localhost';
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPath.toString());
  renderHook(() => useLogin());
  expect(history.push).toHaveBeenCalledWith(samlIdpPath, true);
});

it('non-base domain redirects with base domain for a matching "/enterprise/saml-idp/sso"', async () => {
  const samlIdpPath = new URL('http://different-base' + cfg.routes.samlIdpSso);
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPath.toString());
  renderHook(() => useLogin());
  const expectedPath = new URL('http://localhost' + cfg.routes.samlIdpSso);
  expect(history.push).toHaveBeenCalledWith(expectedPath, true);
});

it('base domain with different path is redirected to root', async () => {
  const nonSamlIdpPath = new URL('http://localhost/web/cluster/name/resources');
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(nonSamlIdpPath.toString());
  renderHook(() => useLogin());
  expect(history.replace).toHaveBeenCalledWith('/web');
});

it('invalid session does nothing', async () => {
  const samlIdpPathWithDifferentBase = new URL(
    'http://different-base' + cfg.routes.samlIdpSso
  );
  jest
    .spyOn(history, 'getRedirectParam')
    .mockReturnValue(samlIdpPathWithDifferentBase.toString());
  jest.spyOn(session, 'isValid').mockImplementation(() => false);
  renderHook(() => useLogin());
  expect(history.replace).not.toHaveBeenCalled();
  expect(history.push).not.toHaveBeenCalled();
});
