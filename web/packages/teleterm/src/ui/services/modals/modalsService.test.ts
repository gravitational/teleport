/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { ModalsService, DialogClusterConnect } from './modalsService';

const rootCluster = makeRootCluster();

function makeDialogClusterConnect(): DialogClusterConnect {
  return {
    kind: 'cluster-connect',
    clusterUri: rootCluster.uri,
    reason: undefined,
    prefill: undefined,
    onSuccess: jest.fn(),
    onCancel: jest.fn(),
  };
}

test('closing all dialogs', () => {
  const dialogClusterConnect1 = makeDialogClusterConnect();
  const dialogClusterConnect2 = makeDialogClusterConnect();
  const modalsService = new ModalsService();

  modalsService.openRegularDialog(dialogClusterConnect1);
  modalsService.openImportantDialog(dialogClusterConnect2);
  expect(modalsService.state.regular).toStrictEqual(dialogClusterConnect1);
  expect(modalsService.state.important).toHaveLength(1);
  expect(modalsService.state.important[0].dialog).toStrictEqual(
    dialogClusterConnect2
  );

  modalsService.cancelAndCloseAll();
  expect(modalsService.state.regular).toStrictEqual(undefined);
  expect(modalsService.state.important).toHaveLength(0);
  expect(dialogClusterConnect1.onCancel).toHaveBeenCalledTimes(1);
  expect(dialogClusterConnect2.onCancel).toHaveBeenCalledTimes(1);
});

test('closing regular dialog with abort signal', () => {
  const dialogClusterConnect = makeDialogClusterConnect();
  const modalsService = new ModalsService();
  const controller = new AbortController();

  modalsService.openRegularDialog(dialogClusterConnect, controller.signal);
  expect(modalsService.state.regular).toStrictEqual(dialogClusterConnect);
  controller.abort();
  expect(modalsService.state.regular).toStrictEqual(undefined);
  expect(dialogClusterConnect.onCancel).toHaveBeenCalledTimes(1);
});

test('closing important dialog with abort signal', () => {
  const dialogClusterConnect1 = makeDialogClusterConnect();
  const dialogClusterConnect2 = makeDialogClusterConnect();
  const modalsService = new ModalsService();
  const controller1 = new AbortController();
  const controller2 = new AbortController();

  modalsService.openImportantDialog(dialogClusterConnect1, controller1.signal);
  modalsService.openImportantDialog(dialogClusterConnect2, controller2.signal);
  expect(modalsService.state.important).toHaveLength(2);
  expect(modalsService.state.important[0].dialog).toStrictEqual(
    dialogClusterConnect1
  );
  expect(modalsService.state.important[1].dialog).toStrictEqual(
    dialogClusterConnect2
  );

  controller1.abort();
  expect(modalsService.state.important).toHaveLength(1);
  expect(modalsService.state.important[0].dialog).toStrictEqual(
    dialogClusterConnect2
  );
  expect(dialogClusterConnect1.onCancel).toHaveBeenCalledTimes(1);

  controller2.abort();
  expect(modalsService.state.important).toHaveLength(0);
  expect(dialogClusterConnect2.onCancel).toHaveBeenCalledTimes(1);
});

test('dialogs opened with aborted signal return immediately', () => {
  const dialogClusterConnect = makeDialogClusterConnect();
  const modalsService = new ModalsService();
  const controller = new AbortController();
  controller.abort();

  modalsService.openRegularDialog(dialogClusterConnect, controller.signal);
  expect(modalsService.state.regular).toStrictEqual(undefined);
  expect(dialogClusterConnect.onCancel).toHaveBeenCalledTimes(1);

  modalsService.openImportantDialog(dialogClusterConnect, controller.signal);
  expect(modalsService.state.important).toHaveLength(0);
  expect(dialogClusterConnect.onCancel).toHaveBeenCalledTimes(2);
});
