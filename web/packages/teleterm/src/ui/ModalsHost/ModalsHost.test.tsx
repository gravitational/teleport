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

import { act } from '@testing-library/react';

import { render, screen } from 'design/utils/testing';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DialogClusterConnect,
  DialogDocumentsReopen,
  DialogHardwareKeyTouch,
  ModalsService,
} from 'teleterm/ui/services/modals';

import ModalsHost from './ModalsHost';

const clusterConnectDialog: DialogClusterConnect = {
  kind: 'cluster-connect',
  clusterUri: '/clusters/foo',
  reason: undefined,
  onCancel: () => {},
  onSuccess: () => {},
  prefill: undefined,
};

const hardwareKeyTouchDialog: DialogHardwareKeyTouch = {
  kind: 'hardware-key-touch',
  req: {
    rootClusterUri: '/clusters/foo',
  },
  onCancel: () => {},
};

const documentsReopenDialog: DialogDocumentsReopen = {
  kind: 'documents-reopen',
  rootClusterUri: '/clusters/foo',
  numberOfDocuments: 2,
  onConfirm: () => {},
  onCancel: () => {},
};

jest.mock('teleterm/ui/ClusterConnect', () => ({
  ClusterConnect: props => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind="cluster-connect"
      data-dialog-is-hidden={props.hidden}
    />
  ),
}));

jest.mock('teleterm/ui/ModalsHost/modals/HardwareKeys/Touch', () => ({
  Touch: props => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind="hardware-key-touch"
      data-dialog-is-hidden={props.hidden}
    />
  ),
}));

jest.mock('teleterm/ui/DocumentsReopen', () => ({
  DocumentsReopen: props => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind="documents-reopen"
      data-dialog-is-hidden={props.hidden}
    />
  ),
}));

test('the important dialog is rendered above the regular dialog', () => {
  const importantDialog = clusterConnectDialog;
  const regularDialog = documentsReopenDialog;

  const modalsService = new ModalsService();
  modalsService.openRegularDialog(regularDialog);
  modalsService.openImportantDialog(importantDialog);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  render(
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );

  // The DOM testing library doesn't really allow us to test actual visibility in terms of the order
  // of rendering, so we have to fall back to manually checking items in the array.
  // https://github.com/testing-library/react-testing-library/issues/313
  const dialogs = screen.queryAllByTestId('mocked-dialog');

  // The important dialog should be after the regular dialog in the DOM so that it's shown over the
  // regular dialog.
  expect(dialogs[0]).toHaveAttribute('data-dialog-kind', regularDialog.kind);
  expect(dialogs[0]).toHaveAttribute('data-dialog-is-hidden', 'true');
  expect(dialogs[1]).toHaveAttribute('data-dialog-kind', importantDialog.kind);
  expect(dialogs[1]).toHaveAttribute('data-dialog-is-hidden', 'false');
});

test('the second important dialog is rendered above the first important dialog', () => {
  const importantDialog1 = clusterConnectDialog;
  const importantDialog2 = hardwareKeyTouchDialog;

  const modalsService = new ModalsService();
  modalsService.openImportantDialog(importantDialog1);
  const { closeDialog: closeImportantDialog2 } =
    modalsService.openImportantDialog(importantDialog2);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  render(
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );

  // The DOM testing library doesn't really allow us to test actual visibility in terms of the order
  // of rendering, so we have to fall back to manually checking items in the array.
  // https://github.com/testing-library/react-testing-library/issues/313
  let dialogs = screen.queryAllByTestId('mocked-dialog');

  // The second important dialog should be after the regular dialog in the DOM so that it's shown over the
  // first important dialog.
  // On top of that, only the important dialog on the top should be visible.
  expect(dialogs[0]).toHaveAttribute('data-dialog-kind', importantDialog1.kind);
  expect(dialogs[0]).toHaveAttribute('data-dialog-is-hidden', 'true');
  expect(dialogs[1]).toHaveAttribute('data-dialog-kind', importantDialog2.kind);
  expect(dialogs[1]).toHaveAttribute('data-dialog-is-hidden', 'false');

  act(() => closeImportantDialog2());

  dialogs = screen.queryAllByTestId('mocked-dialog');

  // The dialog previously on top was closed.
  // Now the first dialog is visible.
  expect(dialogs[0]).toHaveAttribute('data-dialog-kind', importantDialog1.kind);
  expect(dialogs[0]).toHaveAttribute('data-dialog-is-hidden', 'false');
});
