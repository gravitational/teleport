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

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DialogClusterLogout,
  DialogDocumentsReopen,
  ModalsService,
} from 'teleterm/ui/services/modals';

import ModalsHost from './ModalsHost';

export default {
  title: 'Teleterm/ModalsHost',
};

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

const importantDialog = clusterLogoutDialog;
const regularDialog = documentsReopenDialog;

export const RegularModal = () => {
  const modalsService = new ModalsService();
  modalsService.openRegularDialog(regularDialog);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  return (
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );
};

export const ImportantModal = () => {
  const modalsService = new ModalsService();
  modalsService.openImportantDialog(importantDialog);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  return (
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );
};

export const ImportantAndRegularModal = () => {
  const modalsService = new ModalsService();
  modalsService.openRegularDialog(regularDialog);
  modalsService.openImportantDialog(importantDialog);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  return (
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );
};
