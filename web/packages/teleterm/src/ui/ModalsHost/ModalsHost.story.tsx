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

import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import {
  DialogDocumentsReopen,
  DialogHardwareKeyPin,
  DialogHardwareKeyTouch,
  ModalsService,
} from 'teleterm/ui/services/modals';

import ModalsHost from './ModalsHost';

export default {
  title: 'Teleterm/ModalsHost',
};

const hardwareKeyTouchDialog: DialogHardwareKeyTouch = {
  kind: 'hardware-key-touch',
  req: {
    rootClusterUri: '/clusters/foo',
  },
  onCancel: () => {},
};

const hardwareKeyPinDialog: DialogHardwareKeyPin = {
  kind: 'hardware-key-pin',
  req: {
    rootClusterUri: '/clusters/foo',
    pinOptional: false,
  },
  onSuccess: () => {},
  onCancel: () => {},
};

const documentsReopenDialog: DialogDocumentsReopen = {
  kind: 'documents-reopen',
  rootClusterUri: '/clusters/foo',
  numberOfDocuments: 1,
  onConfirm: () => {},
  onDiscard: () => {},
  onCancel: () => {},
};

const importantDialog = hardwareKeyTouchDialog;
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

export const TwoImportantModals = () => {
  const modalsService = new ModalsService();
  modalsService.openImportantDialog(importantDialog);
  modalsService.openImportantDialog(hardwareKeyPinDialog);

  const appContext = new MockAppContext();
  appContext.modalsService = modalsService;

  return (
    <MockAppContextProvider appContext={appContext}>
      <ModalsHost />
    </MockAppContextProvider>
  );
};
