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

import { SendPendingHeadlessAuthenticationRequest } from 'teleterm/services/tshdEvents';
import { MainProcessClient } from 'teleterm/types';
import { ModalsService } from 'teleterm/ui/services/modals';
import { ConfigService } from 'teleterm/services/config';

import type * as types from 'teleterm/services/tshd/types';

export class HeadlessAuthenticationService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private modalsService: ModalsService,
    private tshClient: types.TshClient,
    private configService: ConfigService
  ) {}

  sendPendingHeadlessAuthentication(
    request: SendPendingHeadlessAuthenticationRequest,
    onRequestCancelled: (callback: () => void) => void
  ): Promise<void> {
    const skipConfirm = this.configService.get('headless.skipConfirm').value;

    // If the user wants to skip the confirmation step, then don't force the window.
    // Instead, they can just tap their blinking yubikey with the window in the background.
    if (!skipConfirm) {
      this.mainProcessClient.forceFocusWindow();
    }

    return new Promise(resolve => {
      const { closeDialog } = this.modalsService.openImportantDialog({
        kind: 'headless-authn',
        rootClusterUri: request.rootClusterUri,
        headlessAuthenticationId: request.headlessAuthenticationId,
        headlessAuthenticationClientIp: request.headlessAuthenticationClientIp,
        skipConfirm: skipConfirm,
        onSuccess: () => resolve(),
        onCancel: () => resolve(),
      });

      onRequestCancelled(closeDialog);
    });
  }

  async updateHeadlessAuthenticationState(
    params: types.UpdateHeadlessAuthenticationStateParams,
    abortSignal: types.TshAbortSignal
  ): Promise<void> {
    return this.tshClient.updateHeadlessAuthenticationState(
      params,
      abortSignal
    );
  }
}
