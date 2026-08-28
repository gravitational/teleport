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

import Logger, { NullService } from 'teleterm/logger';
import AppContext from 'teleterm/ui/appContext';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { Dialog } from 'teleterm/ui/services/modals';

import { showStartupModalsAndNotifications } from './showStartupModalsAndNotifications';

beforeAll(() => {
  Logger.init(new NullService());
});

describe('usage reporting dialogs', () => {
  test('only `usage-data` is shown on a fresh run', async () => {
    const mockedAppContext = new MockAppContext();
    mockOpenRegularDialog(mockedAppContext, dialog => {
      if (dialog.kind === 'usage-data') {
        dialog.onAllow();
      }
    });

    await showStartupModalsAndNotifications(mockedAppContext);

    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage-data' }));
    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'user-job-role' })
    );
  });

  test('only `user-job-role` is shown when reporting was enabled earlier', async () => {
    const mockedAppContext = new MockAppContext();
    mockUsageReportingEnabled(mockedAppContext, { enabled: true });
    mockOpenRegularDialog(mockedAppContext, dialog => {
      if (dialog.kind === 'user-job-role') {
        dialog.onSend('Engineer');
      }
    });

    await showStartupModalsAndNotifications(mockedAppContext);

    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage-data' }));
    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).toHaveBeenCalledWith(expect.objectContaining({ kind: 'user-job-role' }));
  });

  test('no dialog is shown when reporting was disabled earlier', async () => {
    const mockedAppContext = new MockAppContext();
    mockUsageReportingEnabled(mockedAppContext, { enabled: false });
    mockOpenRegularDialog(mockedAppContext);

    await showStartupModalsAndNotifications(mockedAppContext);

    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage-data' }));
    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'user-job-role' })
    );
  });

  test('no dialog is shown when reporting was enabled and user was asked about job role earlier', async () => {
    const mockedAppContext = new MockAppContext();
    mockUsageReportingEnabled(mockedAppContext, { enabled: true });
    jest
      .spyOn(mockedAppContext.statePersistenceService, 'getUsageReportingState')
      .mockImplementation(() => ({ askedForUserJobRole: true }));
    mockOpenRegularDialog(mockedAppContext);

    await showStartupModalsAndNotifications(mockedAppContext);

    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage-data' }));
    expect(
      mockedAppContext.modalsService.openRegularDialog
    ).not.toHaveBeenCalledWith(
      expect.objectContaining({ kind: 'user-job-role' })
    );
  });
});

test('no dialog is shown when config file did not load properly', async () => {
  const mockedAppContext = new MockAppContext();
  jest
    .spyOn(mockedAppContext.mainProcessClient.configService, 'getConfigError')
    .mockImplementation(() => ({ source: 'file-loading', error: new Error() }));
  mockOpenRegularDialog(mockedAppContext);

  await showStartupModalsAndNotifications(mockedAppContext);

  expect(
    mockedAppContext.modalsService.openRegularDialog
  ).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'usage-data' }));
  expect(
    mockedAppContext.modalsService.openRegularDialog
  ).not.toHaveBeenCalledWith(
    expect.objectContaining({ kind: 'user-job-role' })
  );
});

function mockUsageReportingEnabled(
  mockedAppContext: AppContext,
  options: { enabled: boolean }
) {
  jest
    .spyOn(mockedAppContext.mainProcessClient.configService, 'get')
    .mockImplementation(key => {
      if (key === 'usageReporting.enabled') {
        return {
          value: options.enabled,
          metadata: {
            isStored: true,
          },
        };
      }
    });
}

function mockOpenRegularDialog(
  mockedAppContext: AppContext,
  implementation?: (dialog: Dialog) => void
) {
  jest
    .spyOn(mockedAppContext.modalsService, 'openRegularDialog')
    .mockImplementation(dialog => {
      implementation?.(dialog);
      dialog['onCancel']?.();
      return {
        closeDialog() {},
      };
    });
}
