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

import { debounce } from 'shared/utils/highbar';

import { ConfigService } from 'teleterm/services/config';
import { TshdClient, VnetClient } from 'teleterm/services/tshd/createClient';
import {
  ElectronGlobals,
  MainProcessClient,
  TshdEventContextBridgeService,
} from 'teleterm/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { ConnectMyComputerService } from 'teleterm/ui/services/connectMyComputer';
import { FileTransferService } from 'teleterm/ui/services/fileTransferClient';
import { HeadlessAuthenticationService } from 'teleterm/ui/services/headlessAuthn/headlessAuthnService';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { ModalsService } from 'teleterm/ui/services/modals';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { ReloginService } from 'teleterm/ui/services/relogin/reloginService';
import { ResourcesService } from 'teleterm/ui/services/resources';
import { StatePersistenceService } from 'teleterm/ui/services/statePersistence';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { TshdNotificationsService } from 'teleterm/ui/services/tshdNotifications/tshdNotificationService';
import { UsageService } from 'teleterm/ui/services/usage';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService/workspacesService';
import { IAppContext, UnexpectedVnetShutdownListener } from 'teleterm/ui/types';

import { CommandLauncher } from './commandLauncher';
import { createTshdEventsContextBridgeService } from './tshdEvents';

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
  tshd: TshdClient;
  vnet: VnetClient;
  /**
   * setupTshdEventContextBridgeService adds a context-bridge-compatible version of a gRPC service
   * that's going to be called every time a client makes a particular RPC to the tshd events
   * service. The service receives requests converted to simple JS objects since classes cannot be
   * passed through the context bridge.
   *
   * See the JSDoc for TshdEventContextBridgeService for more details.
   */
  setupTshdEventContextBridgeService: (
    service: TshdEventContextBridgeService
  ) => void;
  getPathForFile: (file: File) => string;
  reloginService: ReloginService;
  tshdNotificationsService: TshdNotificationsService;
  headlessAuthenticationService: HeadlessAuthenticationService;
  usageService: UsageService;
  configService: ConfigService;
  connectMyComputerService: ConnectMyComputerService;
  private _unexpectedVnetShutdownListener:
    | UnexpectedVnetShutdownListener
    | undefined;

  constructor(config: ElectronGlobals) {
    const { tshClient, ptyServiceClient, mainProcessClient } = config;
    this.tshd = tshClient;
    this.vnet = config.vnetClient;
    this.setupTshdEventContextBridgeService =
      config.setupTshdEventContextBridgeService;
    this.mainProcessClient = mainProcessClient;
    this.notificationsService = new NotificationsService();
    this.configService = this.mainProcessClient.configService;
    this.getPathForFile = config.getPathForFile;
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
  }

  async pullInitialState(): Promise<void> {
    this.setupTshdEventContextBridgeService(
      createTshdEventsContextBridgeService(this)
    );

    this.notifyMainProcessAboutClusterListChanges();
    void this.clustersService.syncGatewaysAndCatchErrors();
    await this.clustersService.syncRootClustersAndCatchErrors();
    this.workspacesService.restorePersistedState();
    // The app has been initialized (callbacks are set up, state is restored).
    // The UI is visible.
    this.workspacesService.markAsInitialized();
  }

  /**
   * addUnexpectedVnetShutdownListener sets the listener and returns a cleanup function which
   * removes the listener.
   */
  addUnexpectedVnetShutdownListener(
    listener: UnexpectedVnetShutdownListener
  ): () => void {
    this._unexpectedVnetShutdownListener = listener;

    return () => {
      this._unexpectedVnetShutdownListener = undefined;
    };
  }

  /**
   * unexpectedVnetShutdownListener gets called by tshd events service when it gets a report about
   * said shutdown from tsh daemon.
   *
   * The communication between tshd events service and VnetContext is done through a callback on
   * AppContext. That's because tshd events service lives outside of React but within the same
   * process (renderer).
   */
  // To force callsites to use addUnexpectedVnetShutdownListener instead of setting the property
  // directly on appContext, we use a getter which exposes a private property.
  get unexpectedVnetShutdownListener(): UnexpectedVnetShutdownListener {
    return this._unexpectedVnetShutdownListener;
  }

  private notifyMainProcessAboutClusterListChanges() {
    // Debounce the notifications sent to the main process so that we don't unnecessarily send more
    // than one notification per frame. The main process doesn't need to be notified absolutely
    // immediately after a change in the cluster list.
    //
    // The clusters map in ClustersService gets updated a bunch of times during the start of the
    // app. After each update, the renderer tells the main process to refresh the list. The main
    // process sends a request to list root clusters and cancels any pending ones. Debouncing here
    // helps to minimize those cancellations.
    const refreshClusterList = debounce(
      this.mainProcessClient.refreshClusterList,
      16
    );
    this.clustersService.subscribeWithSelector(
      state => state.clusters,
      refreshClusterList
    );
  }
}
