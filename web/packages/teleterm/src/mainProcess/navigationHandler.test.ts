/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { dialog, shell, WebContents } from 'electron';

import Logger, { NullService } from 'teleterm/logger';
import { makeRuntimeSettings } from 'teleterm/mainProcess/fixtures/mocks';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { registerNavigationHandlers } from './navigationHandler';

beforeAll(() => {
  Logger.init(new NullService());
});

beforeEach(() => {
  jest.resetAllMocks();
});

jest.mock('electron', () => ({
  dialog: {
    showErrorBox: jest.fn(),
  },
  shell: {
    openExternal: jest.fn(),
  },
}));

const cluster = makeRootCluster();

describe('opening links to', () => {
  test.each([
    {
      name: 'Teleport',
      url: 'https://goteleport.com/',
      allowed: true,
    },
    {
      name: 'Gravitational GitHub',
      url: 'https://github.com/gravitational/',
      allowed: true,
    },
    {
      name: 'cluster SSO host',
      // comes from makeRootCluster
      url: 'https://example.auth0.com/some-path',
      allowed: true,
    },
    {
      name: 'cluster proxy host',
      // comes from makeRootCluster
      url: 'https://teleport-local.com:3080/some-path',
      allowed: true,
    },
    {
      name: 'non-HTTPS URLs',
      url: 'http://goteleport.com',
      allowed: false,
    },
    {
      name: 'non-Gravitational GitHub',
      url: 'https://github.com/abc',
      allowed: false,
    },
    {
      name: 'arbitrary URLs',
      url: 'https://google.com',
      allowed: false,
    },
  ])('$name', test => {
    let handler: Parameters<WebContents['setWindowOpenHandler']>[0];
    registerNavigationHandlers(
      {
        setWindowOpenHandler: d => {
          handler = d;
        },
        on: jest.fn(),
      },
      makeRuntimeSettings(),
      { getRootClusters: () => [cluster] },
      new Logger()
    );

    const result = handler({
      url: test.url,
      frameName: '',
      features: '',
      disposition: 'default',
      referrer: undefined,
    });

    expect(result).toEqual({ action: 'deny' });
    /* eslint-disable jest/no-conditional-expect */
    if (test.allowed) {
      expect(shell.openExternal).toHaveBeenCalledWith(test.url);
      expect(dialog.showErrorBox).not.toHaveBeenCalled();
    } else {
      expect(shell.openExternal).not.toHaveBeenCalled();
      expect(dialog.showErrorBox).toHaveBeenCalledWith(
        'Cannot open this link',
        'The domain does not match any of the allowed domains. Check main.log for more details.'
      );
    }
  });
});
