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

import { ConfigService } from 'teleterm/services/config';
import type { CloneableAbortSignal, TshdClient } from 'teleterm/services/tshd';
import type * as types from 'teleterm/services/tshd/types';
import { SendPendingHeadlessAuthenticationRequest } from 'teleterm/services/tshdEvents';
import { MainProcessClient } from 'teleterm/types';
import { ModalsService } from 'teleterm/ui/services/modals';

export class HeadlessAuthenticationService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private modalsService: ModalsService,
    private tshClient: TshdClient,
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
    params: types.UpdateHeadlessAuthenticationStateRequest,
    abortSignal: CloneableAbortSignal
  ): Promise<void> {
    await this.tshClient.updateHeadlessAuthenticationState(params, {
      abort: abortSignal,
    });
  }
}
