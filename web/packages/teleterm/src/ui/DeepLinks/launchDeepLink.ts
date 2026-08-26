/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
  AuthenticateWebDeviceDeepURL,
  ConnectMyComputerDeepURL,
  DeepURL,
  VnetDeepURL,
} from 'shared/deepLinks';

import { DeepLinkParseResult } from 'teleterm/deepLinks';
import { IAppContext } from 'teleterm/ui/types';
import { RootClusterUri, routing } from 'teleterm/ui/uri';
import { VnetContext } from 'teleterm/ui/Vnet';

/**
 * AuxContext holds all the extra things that launchDeepLink needs that come from React contexts.
 */
export type AuxContext = {
  vnet: Pick<VnetContext, 'isSupported'>;
};

/**
 * Performs the business logic associated with the given deep link.
 */
export async function launchDeepLink(
  appCtx: IAppContext,
  auxCtx: AuxContext,
  result: DeepLinkParseResult
): Promise<void> {
  const { notificationsService, modalsService } = appCtx;

  if (result.status === 'error') {
    let reason: string;
    switch (result.reason) {
      case 'unknown-protocol': {
        reason = `The URL of the link is of an unknown protocol.`;
        break;
      }
      case 'unsupported-url': {
        reason =
          'The received link does not point at a resource or an action that can be launched from a link. ' +
          'Either this version of Teleport Connect does not support it or the link is incorrect.';
        break;
      }
      case 'malformed-url': {
        reason = `The URL of the link appears to be malformed. ${result.error.message}`;
        break;
      }
      default: {
        result satisfies never;
      }
    }

    notificationsService.notifyWarning({
      title: 'Cannot open the link',
      description: reason,
    });
    return;
  }

  // Before we start, let's close any open dialogs, for a few reasons:
  // 1. Activating a deep link may require changing the workspace, and we don't
  // want to see dialogs from the previous one.
  // 2. A login dialog could be covered by an important dialog.
  // 3. The user could be confused, since Connect My Computer or Authorize Web
  // Session documents would be displayed below a dialog.
  modalsService.cancelAndCloseAll();

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
      await launchConnectMyComputer(appCtx, result.url);
      break;
    }
    case '/authenticate_web_device': {
      await askAuthorizeDeviceTrust(appCtx, result.url);
      break;
    }
    case '/vnet': {
      await openVnetInfoDoc(appCtx, auxCtx, result.url);
      break;
    }
    default: {
      result.url satisfies never;
    }
  }
}

/**
 * askAuthorizeDeviceTrust opens a document asking the user if they'd like to authorize
 * a web session with device trust. If confirmed, the web session will be upgraded and the
 * user will be directed back to the web UI.
 */
async function askAuthorizeDeviceTrust(
  appCtx: IAppContext,
  url: AuthenticateWebDeviceDeepURL
): Promise<void> {
  const { workspacesService } = appCtx;
  const { id, token, redirect_uri } = url.searchParams;

  const result = await loginAndSetActiveWorkspace(appCtx, url);
  if (!result.isAtDesiredWorkspace) {
    return;
  }

  const { rootClusterUri } = result;
  const documentService =
    workspacesService.getWorkspaceDocumentService(rootClusterUri);
  const doc = documentService.createAuthorizeWebSessionDocument({
    rootClusterUri,
    webSessionRequest: {
      id,
      token,
      username: url.username,
      redirectUri: redirect_uri,
    },
  });
  documentService.add(doc);
  documentService.open(doc.uri);
}

/**
 * launchConnectMyComputer opens a Connect My Computer tab in the cluster workspace that the URL
 * points to.
 */
async function launchConnectMyComputer(
  appCtx: IAppContext,
  url: ConnectMyComputerDeepURL
): Promise<void> {
  const { notificationsService, workspacesService, mainProcessClient } = appCtx;
  if (mainProcessClient.getRuntimeSettings().platform === 'win32') {
    notificationsService.notifyWarning(
      'Connect My Computer is not supported on Windows.'
    );
    return;
  }

  const result = await loginAndSetActiveWorkspace(appCtx, url);

  if (!result.isAtDesiredWorkspace) {
    return;
  }

  const { rootClusterUri } = result;

  workspacesService
    .getWorkspaceDocumentService(rootClusterUri)
    .openConnectMyComputerDocument({ rootClusterUri });
}

/**
 * loginAndSetActiveWorkspace will set the relevant cluster if it is in the app and, if not,
 * it opens a login dialog with cluster address and username prefilled from the URL.
 */
async function loginAndSetActiveWorkspace(
  { clustersService, modalsService, workspacesService }: IAppContext,
  url: DeepURL
): Promise<
  | {
      isAtDesiredWorkspace: false;
    }
  | {
      isAtDesiredWorkspace: true;
      rootClusterUri: RootClusterUri;
    }
> {
  const currentlyActiveWorkspace = workspacesService.getRootClusterUri();
  // If we closed the dialog to reopen documents when launching a deep link,
  // setting the active workspace again will reopen it.
  const reopenCurrentlyActiveWorkspace = async () => {
    if (currentlyActiveWorkspace) {
      await workspacesService.setActiveWorkspace(currentlyActiveWorkspace);
    }
  };
  const rootClusterId = url.hostname;
  const clusterAddress = url.host;
  const prefill = {
    clusterAddress,
    username: url.username,
  };

  const rootClusterUri = routing.getClusterUri({ rootClusterId });
  const rootCluster = clustersService.findCluster(rootClusterUri);

  if (!rootCluster) {
    const { canceled } = await new Promise<{ canceled: boolean }>(resolve => {
      modalsService.openRegularDialog({
        kind: 'cluster-connect',
        clusterUri: undefined,
        reason: undefined,
        prefill,
        onSuccess: () => resolve({ canceled: false }),
        onCancel: () => resolve({ canceled: true }),
      });
    });

    if (canceled) {
      await reopenCurrentlyActiveWorkspace();
      return {
        isAtDesiredWorkspace: false,
      };
    }
  }

  const { isAtDesiredWorkspace } = await workspacesService.setActiveWorkspace(
    rootClusterUri,
    // prefill has to be passed here as wellin case the cluster is in the state (so the call
    // to open cluster-connect above will be skipped) but there's no active cert. In that
    // case, WorkspacesService will open cluster-connect itself with just the login step.
    prefill
  );

  if (isAtDesiredWorkspace) {
    return { isAtDesiredWorkspace: true, rootClusterUri };
  }

  await reopenCurrentlyActiveWorkspace();
  return { isAtDesiredWorkspace: false };
}

async function openVnetInfoDoc(
  appCtx: IAppContext,
  { vnet }: AuxContext,
  url: VnetDeepURL
): Promise<void> {
  const { notificationsService, workspacesService } = appCtx;
  if (!vnet.isSupported) {
    notificationsService.notifyWarning(
      'VNet is not supported on this platform.'
    );
    return;
  }

  const result = await loginAndSetActiveWorkspace(appCtx, url);

  if (!result.isAtDesiredWorkspace) {
    return;
  }

  const { rootClusterUri } = result;
  const docsService =
    workspacesService.getWorkspaceDocumentService(rootClusterUri);
  docsService.openExistingOrAddNew(
    d => d.kind === 'doc.vnet_info',
    () => docsService.createVnetInfoDocument({ rootClusterUri })
  );
}
