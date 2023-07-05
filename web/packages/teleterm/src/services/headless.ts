/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { MainProcessClient } from 'teleterm/types';
import { HeadlessAuthenticationRequest } from 'teleterm/services/tshdEvents';
import { ModalsService } from 'teleterm/ui/services/modals';
import { ClustersService } from 'teleterm/ui/services/clusters';

export class HeadlessAuthenticationService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private modalsService: ModalsService,
    // private clustersService: ClustersService
  ) {}

  headlessAuthentication(
    // request: HeadlessAuthenticationRequest,
    onRequestCancelled: (callback: () => void) => void
  ): Promise<void> {
    this.mainProcessClient.forceFocusWindow();

    return new Promise((resolve, reject) => {
      const { closeDialog } = this.modalsService.openImportantDialog({
        kind: 'prompt-webauthn',
        onSuccess: () => resolve(),
        onCancel: () =>
          reject(new Error('Webauthn challenge was cancelled by user')),
      });

      onRequestCancelled(closeDialog);
    });
  }
}