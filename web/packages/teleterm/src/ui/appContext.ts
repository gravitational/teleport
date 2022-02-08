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

import { MainProcessClient, ElectronGlobals } from 'teleterm/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ModalsService } from 'teleterm/ui/services/modals';
import { DocumentsService } from 'teleterm/ui/services/docs';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { ConnectionTrackerService } from 'teleterm/ui/services/connectionTracker';
import { QuickInputService } from 'teleterm/ui/services/quickInput';
import { WorkspaceService } from 'teleterm/ui/services/workspace';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { CommandLauncher } from './commandLauncher';

export default class AppContext {
  clustersService: ClustersService;
  modalsService: ModalsService;
  docsService: DocumentsService;
  terminalsService: TerminalsService;
  keyboardShortcutsService: KeyboardShortcutsService;
  quickInputService: QuickInputService;
  workspaceService: WorkspaceService;
  mainProcessClient: MainProcessClient;
  commandLauncher: CommandLauncher;
  connectionTracker: ConnectionTrackerService;

  constructor(config: ElectronGlobals) {
    const { tshClient, ptyServiceClient, mainProcessClient } = config;
    this.mainProcessClient = mainProcessClient;
    this.clustersService = new ClustersService(tshClient);
    this.modalsService = new ModalsService();
    this.terminalsService = new TerminalsService(ptyServiceClient);
    this.docsService = new DocumentsService();

    this.keyboardShortcutsService = new KeyboardShortcutsService(
      this.mainProcessClient.getRuntimeSettings().platform,
      this.mainProcessClient.configService
    );

    this.workspaceService = new WorkspaceService(config.fileStorage);
    this.commandLauncher = new CommandLauncher(this);

    this.quickInputService = new QuickInputService(
      this.commandLauncher,
      this.clustersService
    );

    this.connectionTracker = new ConnectionTrackerService(
      this.workspaceService,
      this.docsService,
      this.clustersService
    );
  }

  async init() {
    await this.clustersService.syncRootClusters();
  }
}
