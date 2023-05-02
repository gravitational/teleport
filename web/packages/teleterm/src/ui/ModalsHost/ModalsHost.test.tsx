/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
