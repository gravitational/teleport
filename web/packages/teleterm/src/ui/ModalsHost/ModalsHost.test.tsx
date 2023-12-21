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

import React from 'react';
import { render, screen } from 'design/utils/testing';

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DialogClusterLogout,
  DialogDocumentsReopen,
  ModalsService,
} from 'teleterm/ui/services/modals';

import ModalsHost from './ModalsHost';

const clusterLogoutDialog: DialogClusterLogout = {
  kind: 'cluster-logout',
  clusterUri: '/clusters/foo',
  clusterTitle: 'Foo',
};

const documentsReopenDialog: DialogDocumentsReopen = {
  kind: 'documents-reopen',
  rootClusterUri: '/clusters/foo',
  numberOfDocuments: 2,
  onConfirm: () => {},
  onCancel: () => {},
};

jest.mock('teleterm/ui/ClusterLogout', () => ({
  ClusterLogout: () => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind={clusterLogoutDialog.kind}
    ></div>
  ),
}));

jest.mock('teleterm/ui/DocumentsReopen', () => ({
  DocumentsReopen: () => (
    <div
      data-testid="mocked-dialog"
      data-dialog-kind={documentsReopenDialog.kind}
    ></div>
  ),
}));

test('the important dialog is rendered above the regular dialog', () => {
  const importantDialog = clusterLogoutDialog;
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
  expect(dialogs[1]).toHaveAttribute('data-dialog-kind', importantDialog.kind);
});
