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

import Logger from 'teleterm/logger';
import { isTshdRpcError } from 'teleterm/services/tshd/cloneableClient';
import { IAppContext } from 'teleterm/ui/types';
import { ClusterOrResourceUri, RootClusterUri, routing } from 'teleterm/ui/uri';

const logger = new Logger('retryWithRelogin');

let pendingLoginDialog: Promise<void> | undefined;

/**
 * `retryWithRelogin` executes `actionToRetry`. If `actionToRetry` throws an error, it checks if the
 * error can be resolved by the user logging in, according to metadata returned from the tshd
 * client.
 *
 * If that's the case, it checks if the user is still looking at the relevant UI (the `isUiActive`
 * argument) and if so, it shows a login modal. After the user successfully logs in, it calls
 * `actionToRetry` again.
 *
 * `retryWithRelogin` supports concurrent requests.
 * If multiple actions need to show a login modal at the same time,
 * it will be displayed only once, and other actions will wait for it to be resolved.
 *
 * Each place using `retryWithRelogin` must be able to show the error to the user in case the
 * relogin attempt fails. Each place should also offer the user a way to manually retry the action
 * which results in a call to the tshd client.
 *
 * `retryWithRelogin` should wrap calls to the tshd client as tightly as possible. At the moment, it
 * means `actionToRetry` will usually involve calls to `ClustersService`, which so far is the only
 * place that has access to the tshd client.
 *
 * @param resourceUri - The URI used to extract the root cluster URI. That's how we determine the
 * cluster the login modal should use and whether the workspace of that resource is still active.
 */
export async function retryWithRelogin<T>(
  appContext: IAppContext,
  resourceUri: ClusterOrResourceUri,
  actionToRetry: () => Promise<T>
): Promise<T> {
  let retryableErrorFromActionToRetry: Error;
  try {
    return await actionToRetry();
  } catch (error) {
    if (isRetryable(error)) {
      retryableErrorFromActionToRetry = error;
      logger.info(`Activating relogin on error`, error);
    } else {
      throw error;
    }
  }

  const { workspacesService } = appContext;

  if (!workspacesService.doesResourceBelongToActiveWorkspace(resourceUri)) {
    // The error is retryable, but by the time the request has finished, the user has switched to
    // another workspace so they're no longer looking at the relevant UI.
    //
    // Since it might take a few seconds before the execution gets to this point, the user might no
    // longer remember what their intent was, thus showing them a login modal could be disorienting.
    //
    // In that situation, let's just not attempt to relogin and instead let's surface the original
    // error.
    throw retryableErrorFromActionToRetry;
  }

  const rootClusterUri = routing.ensureRootClusterUri(resourceUri);

  if (!pendingLoginDialog) {
    pendingLoginDialog = login(appContext, rootClusterUri).finally(() => {
      pendingLoginDialog = undefined;
    });
  }

  await pendingLoginDialog;

  return await actionToRetry();
}

export function isRetryable(error: unknown): boolean {
  return isTshdRpcError(error) && error.isResolvableWithRelogin;
}

// Notice that we don't differentiate between onSuccess and onCancel. In both cases, we're going to
// retry the action anyway in case the cert was refreshed externally before the modal was closed,
// for example through tsh login.
function login(
  appContext: IAppContext,
  rootClusterUri: RootClusterUri
): Promise<void> {
  return new Promise(resolve => {
    appContext.modalsService.openRegularDialog({
      kind: 'cluster-connect',
      clusterUri: rootClusterUri,
      prefill: undefined,
      reason: undefined,
      onSuccess: () => resolve(),
      onCancel: () => resolve(),
    });
  });
}
