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

import {
  MainProcessClient,
  ElectronGlobals,
  SubscribeToTshdEvent,
} from 'teleterm/types';
import {
  ReloginRequest,
  SendNotificationRequest,
  SendPendingHeadlessAuthenticationRequest,
} from 'teleterm/services/tshdEvents';
import Logger from 'teleterm/logger';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ModalsService } from 'teleterm/ui/services/modals';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService/workspacesService';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { FileTransferService } from 'teleterm/ui/services/fileTransferClient';
import { ReloginService } from 'teleterm/ui/services/relogin/reloginService';
import { TshdNotificationsService } from 'teleterm/ui/services/tshdNotifications/tshdNotificationService';
import { HeadlessAuthenticationService } from 'teleterm/ui/services/headlessAuthn/headlessAuthnService';
import { UsageService } from 'teleterm/ui/services/usage';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { ConnectMyComputerService } from 'teleterm/ui/services/connectMyComputer';
import { ConfigService } from 'teleterm/services/config';
import { IAppContext } from 'teleterm/ui/types';
import { DeepLinksService } from 'teleterm/ui/services/deepLinks';
import { parseDeepLink } from 'teleterm/deepLinks';

import { CommandLauncher } from './commandLauncher';

export default class AppContext implements IAppContext {
  private logger: Logger;
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
  connectMyComputerService: ConnectMyComputerService;
  deepLinksService: DeepLinksService;

  constructor(config: ElectronGlobals) {
    const { tshClient, ptyServiceClient, mainProcessClient } = config;
    this.logger = new Logger('AppContext');
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
    this.connectMyComputerService = new ConnectMyComputerService(
      this.mainProcessClient,
      tshClient
    );
    this.headlessAuthenticationService = new HeadlessAuthenticationService(
      mainProcessClient,
      this.modalsService,
      tshClient,
      this.configService
    );
    this.deepLinksService = new DeepLinksService(
      this.mainProcessClient.getRuntimeSettings(),
      this.clustersService,
      this.workspacesService,
      this.modalsService,
      this.notificationsService
    );
  }

  async pullInitialState(): Promise<void> {
    this.setUpTshdEventSubscriptions();
    this.subscribeToDeepLinkLaunch();
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

    this.subscribeToTshdEvent(
      'sendPendingHeadlessAuthentication',
      ({ request, onCancelled }) => {
        return this.headlessAuthenticationService.sendPendingHeadlessAuthentication(
          request as SendPendingHeadlessAuthenticationRequest,
          onCancelled
        );
      }
    );
  }

  private subscribeToDeepLinkLaunch() {
    this.mainProcessClient.subscribeToDeepLinkLaunch(result => {
      this.deepLinksService.launchDeepLink(result).catch(error => {
        this.logger.error('Error when launching a deep link', error);
      });
    });

    if (process.env.NODE_ENV === 'development') {
      window['deepLinkLaunch'] = (url: string) => {
        const result = parseDeepLink(url);
        this.deepLinksService.launchDeepLink(result).catch(error => {
          this.logger.error('Error when launching a deep link', error);
        });
      };
    }
  }
}
