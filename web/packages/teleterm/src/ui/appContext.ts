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

import {
  MainProcessClient,
  ElectronGlobals,
  SubscribeToTshdEvent,
} from 'teleterm/types';
import {
  ReloginRequest,
  SendNotificationRequest,
  HeadlessAuthenticationRequest,
} from 'teleterm/services/tshdEvents';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ModalsService } from 'teleterm/ui/services/modals';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService/workspacesService';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { FileTransferService } from 'teleterm/ui/services/fileTransferClient';
import { ReloginService } from 'teleterm/services/relogin';
import { TshdNotificationsService } from 'teleterm/services/tshdNotifications';
import { HeadlessAuthenticationService } from 'teleterm/services/headless';
import { UsageService } from 'teleterm/ui/services/usage';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { ConfigService } from 'teleterm/services/config';
import { IAppContext } from 'teleterm/ui/types';

import { CommandLauncher } from './commandLauncher';

export default class AppContext implements IAppContext {
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
  fileTransferService: FileTransferService;
  resourcesService: ResourcesService;
  /**
   * subscribeToTshdEvent lets you add a listener that's going to be called every time a client
   * makes a particular RPC to the tshd events service. The listener receives the request converted
   * to a simple JS object since classes cannot be passed through the context bridge.
   *
   * @param {string} eventName - Name of the event.
   * @param {function} listener - A function that gets called when a client calls the specific
   * event. It accepts an object with two properties:
   *
   * - request is the request payload converted to a simple JS object.
   * - onCancelled is a function which lets you register a callback which will be called when the
   * request gets canceled by the client.
   */
  subscribeToTshdEvent: SubscribeToTshdEvent;
  reloginService: ReloginService;
  tshdNotificationsService: TshdNotificationsService;
  headlessAuthenticationService: HeadlessAuthenticationService;
  usageService: UsageService;
  configService: ConfigService;

  constructor(config: ElectronGlobals) {
    const { tshClient, ptyServiceClient, mainProcessClient } = config;
    this.subscribeToTshdEvent = config.subscribeToTshdEvent;
    this.mainProcessClient = mainProcessClient;
    this.notificationsService = new NotificationsService();
    this.configService = this.mainProcessClient.configService;
    this.usageService = new UsageService(
      tshClient,
      this.configService,
      this.notificationsService,
      clusterUri => this.clustersService.findCluster(clusterUri),
      mainProcessClient.getRuntimeSettings()
    );
    this.fileTransferService = new FileTransferService(
      tshClient,
      this.usageService
    );
    this.resourcesService = new ResourcesService(tshClient);
    this.statePersistenceService = new StatePersistenceService(
      this.mainProcessClient.fileStorage
    );
    this.modalsService = new ModalsService();
    this.clustersService = new ClustersService(
      tshClient,
      this.mainProcessClient,
      this.notificationsService,
      this.usageService
    );
    this.workspacesService = new WorkspacesService(
      this.modalsService,
      this.clustersService,
      this.notificationsService,
      this.statePersistenceService
    );
    this.terminalsService = new TerminalsService(ptyServiceClient);

    this.keyboardShortcutsService = new KeyboardShortcutsService(
      this.mainProcessClient.getRuntimeSettings().platform,
      this.configService
    );

    this.commandLauncher = new CommandLauncher(this);

    this.connectionTracker = new ConnectionTrackerService(
      this.statePersistenceService,
      this.workspacesService,
      this.clustersService
    );

    this.reloginService = new ReloginService(
      mainProcessClient,
      this.modalsService,
      this.clustersService
    );
    this.tshdNotificationsService = new TshdNotificationsService(
      this.notificationsService,
      this.clustersService
    );
    this.headlessAuthenticationService = new HeadlessAuthenticationService(
      mainProcessClient,
      this.modalsService,
    );
  }

  async init(): Promise<void> {
    this.setUpTshdEventSubscriptions();
    this.clustersService.syncGatewaysAndCatchErrors();
    await this.clustersService.syncRootClustersAndCatchErrors();
  }

  private setUpTshdEventSubscriptions() {
    this.subscribeToTshdEvent('relogin', ({ request, onCancelled }) => {
      // The handler for the relogin event should return only after the relogin procedure finishes.
      return this.reloginService.relogin(
        request as ReloginRequest,
        onCancelled
      );
    });

    this.subscribeToTshdEvent('sendNotification', ({ request }) => {
      this.tshdNotificationsService.sendNotification(
        request as SendNotificationRequest
      );
    });

    this.subscribeToTshdEvent('headlessAuthentication', ({ onCancelled }) => {
      return this.headlessAuthenticationService.headlessAuthentication(
        // request as HeadlessAuthentication,
        onCancelled
      );
    });
  }
}
