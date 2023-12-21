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

import { DeepLinkParseResult, DeepURL } from 'teleterm/deepLinks';
import { routing } from 'teleterm/ui/uri';
import { assertUnreachable } from 'teleterm/ui/utils';
import { RuntimeSettings } from 'teleterm/types';
import { ClustersService } from 'teleterm/ui/services/clusters';
import { WorkspacesService } from 'teleterm/ui/services/workspacesService';
import { ModalsService } from 'teleterm/ui/services/modals';
import { NotificationsService } from 'teleterm/ui/services/notifications';

export class DeepLinksService {
  constructor(
    private runtimeSettings: RuntimeSettings,
    private clustersService: ClustersService,
    private workspacesService: WorkspacesService,
    private modalsService: ModalsService,
    private notificationsService: NotificationsService
  ) {}

  /**
   * launchDeepLink either processes a successful result of parsing a deep link URL or shows a
   * notification related to a failed result.
   *
   * It handles failed results because we must show something in the UI even if the clicked deep
   * link was invalid. Otherwise after clicking on an invalid link the OS would focus the app but
   * the UI would remain static, making the app seem broken.
   */
  async launchDeepLink(result: DeepLinkParseResult): Promise<void> {
    if (result.status === 'error') {
      let reason: string;
      switch (result.reason) {
        case 'unknown-protocol': {
          reason = `The URL of the link is of an unknown protocol.`;
          break;
        }
        case 'unsupported-uri': {
          reason =
            'The received link does not point at a resource or an action that can be launched from a link. ' +
            'Either this version of Teleport Connect does not support it or the link is incorrect.';
          break;
        }
        case 'malformed-url': {
          reason = `The URL of the link appears to be malformed.`;
          break;
        }
        default: {
          assertUnreachable(result);
        }
      }

      this.notificationsService.notifyWarning({
        title: 'Cannot open the link',
        description: reason,
      });
      return;
    }

    // launchDeepLink cannot throw if it receives a pathname that doesn't match any supported
    // pathnames. The user might simply be using a version of Connect that doesn't support the given
    // pathname yet. Generally, such cases should be caught outside of DeepLinksService by
    // parseDeepLink and the switch above, if not then it means we have a bug.
    //
    // The code behind mapping the pathname to an action might need to be make more elaborate if we
    // decide to support more advanced use cases with pathnames that contain arguments. See the
    // comment for Path in deepLinks.ts.
    switch (result.url.pathname) {
      case '/connect_my_computer': {
        await this.launchConnectMyComputer(result.url);
      }
    }
  }

  /**
   * launchConnectMyComputer opens a Connect My Computer tab in the cluster workspace that the URL
   * points to. If the relevant cluster is not in the app yet, it opens a login dialog with the
   * cluster address and username prefilled from the URL.
   */
  private async launchConnectMyComputer(url: DeepURL): Promise<void> {
    if (this.runtimeSettings.platform === 'win32') {
      this.notificationsService.notifyWarning(
        'Connect My Computer is not supported on Windows.'
      );
      return;
    }

    const rootClusterId = url.hostname;
    const clusterAddress = url.host;
    const prefill = {
      clusterAddress,
      username: url.username,
    };

    const rootClusterUri = routing.getClusterUri({ rootClusterId });
    const rootCluster = this.clustersService.findCluster(rootClusterUri);

    if (!rootCluster) {
      const { canceled } = await new Promise<{ canceled: boolean }>(resolve => {
        this.modalsService.openRegularDialog({
          kind: 'cluster-connect',
          clusterUri: undefined,
          reason: undefined,
          prefill,
          onSuccess: () => resolve({ canceled: false }),
          onCancel: () => resolve({ canceled: true }),
        });
      });

      if (canceled) {
        return;
      }
    }

    const { isAtDesiredWorkspace } =
      await this.workspacesService.setActiveWorkspace(
        rootClusterUri,
        // prefill has to be passed here as wellin case the cluster is in the state (so the call
        // to open cluster-connect above will be skipped) but there's no active cert. In that
        // case, WorkspacesService will open cluster-connect itself with just the login step.
        prefill
      );

    if (!isAtDesiredWorkspace) {
      return;
    }

    this.workspacesService
      .getWorkspaceDocumentService(rootClusterUri)
      .openConnectMyComputerDocument({ rootClusterUri });
  }
}
