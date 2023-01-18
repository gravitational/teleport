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
