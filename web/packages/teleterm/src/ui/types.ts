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
import * as tshdEventsApi from 'gen-proto-ts/teleport/lib/teleterm/v1/tshd_events_service_pb';

import {
  MainProcessClient,
  TshdEventContextBridgeService,
} from 'teleterm/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ModalsService } from 'teleterm/ui/services/modals';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { FileTransferService } from 'teleterm/ui/services/fileTransferClient';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { ReloginService } from 'teleterm/ui/services/relogin/reloginService';
import { TshdNotificationsService } from 'teleterm/ui/services/tshdNotifications/tshdNotificationService';
import { UsageService } from 'teleterm/ui/services/usage';
import { ConfigService } from 'teleterm/services/config';
import { ConnectMyComputerService } from 'teleterm/ui/services/connectMyComputer';
import { HeadlessAuthenticationService } from 'teleterm/ui/services/headlessAuthn/headlessAuthnService';
import { TshdClient, VnetClient } from 'teleterm/services/tshd';

export interface IAppContext {
  clustersService: ClustersService;
  modalsService: ModalsService;
  notificationsService: NotificationsService;
  terminalsService: TerminalsService;
  keyboardShortcutsService: KeyboardShortcutsService;
  statePersistenceService: StatePersistenceService;
  workspacesService: WorkspacesService;
  mainProcessClient: MainProcessClient;
  commandLauncher: CommandLauncher;
  connectionTracker: ConnectionTrackerService;
  resourcesService: ResourcesService;
  fileTransferService: FileTransferService;
  setupTshdEventContextBridgeService: (
    service: TshdEventContextBridgeService
  ) => void;
  reloginService: ReloginService;
  tshdNotificationsService: TshdNotificationsService;
  usageService: UsageService;
  configService: ConfigService;
  connectMyComputerService: ConnectMyComputerService;
  headlessAuthenticationService: HeadlessAuthenticationService;
  tshd: TshdClient;
  vnet: VnetClient;

  pullInitialState(): Promise<void>;

  /**
   * addUnexpectedVnetShutdownListener sets the listener and returns a cleanup function.
   */
  addUnexpectedVnetShutdownListener: (
    listener: UnexpectedVnetShutdownListener
  ) => () => void;
  /**
   * unexpectedVnetShutdownListener gets called by tshd events service when it gets a report about
   * said shutdown from tsh daemon.
   *
   * The communication between tshd events service and VnetContext is done through a callback on
   * AppContext. That's because tshd events service lives outside of React but within the same
   * process (renderer).
   */
  unexpectedVnetShutdownListener: UnexpectedVnetShutdownListener | undefined;
}

export type UnexpectedVnetShutdownListener = (
  request: tshdEventsApi.ReportUnexpectedVnetShutdownRequest
) => void;
