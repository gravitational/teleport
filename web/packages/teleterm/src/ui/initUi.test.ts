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

import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import Logger, { NullService } from 'teleterm/logger';
import AppContext from 'teleterm/ui/appContext';
import { Dialog } from 'teleterm/ui/services/modals';

import { initUi } from './initUi';

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

    await initUi(mockedAppContext);

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

    await initUi(mockedAppContext);

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

    await initUi(mockedAppContext);

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

    await initUi(mockedAppContext);

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

  await initUi(mockedAppContext);

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
