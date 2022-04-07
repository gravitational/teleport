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

import { MainProcessClient } from 'teleterm/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { ModalsService } from 'teleterm/ui/services/modals';
import { TerminalsService } from 'teleterm/ui/services/terminals';
import { QuickInputService } from 'teleterm/ui/services/quickInput';
import { CommandLauncher } from 'teleterm/ui/commandLauncher';
import { KeyboardShortcutsService } from 'teleterm/ui/services/keyboardShortcuts';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { NotificationsService } from 'teleterm/ui/services/notifications';

export interface IAppContext {
  clustersService: ClustersService;
  modalsService: ModalsService;
  terminalsService: TerminalsService;
  keyboardShortcutsService: KeyboardShortcutsService;
  quickInputService: QuickInputService;
  mainProcessClient: MainProcessClient;
  notificationsService: NotificationsService;
  commandLauncher: CommandLauncher;
  workspacesService: WorkspacesService;
}
