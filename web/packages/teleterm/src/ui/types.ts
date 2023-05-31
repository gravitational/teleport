/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { MainProcessClient, SubscribeToTshdEvent } from 'teleterm/types';
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
import { ReloginService } from 'teleterm/services/relogin';
import { TshdNotificationsService } from 'teleterm/services/tshdNotifications';
import { UsageService } from 'teleterm/ui/services/usage';
import { ConfigService } from 'teleterm/services/config';

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
  subscribeToTshdEvent: SubscribeToTshdEvent;
  reloginService: ReloginService;
  tshdNotificationsService: TshdNotificationsService;
  usageService: UsageService;
  configService: ConfigService;

  init(): Promise<void>;
}
