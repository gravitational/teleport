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

import { routing } from 'teleterm/ui/uri';
import AppContext from 'teleterm/ui/appContext';
import Logger from 'teleterm/logger';

const logger = new Logger('retryWithRelogin');

/**
 * `retryWithRelogin` executes `actionToRetry`. If `actionToRetry` throws an error, it checks if the
 * error can be resolved by the user logging in, according to metadata returned from the tshd
 * client.
 *
 * If that's the case, it checks if the user is still looking at the relevant UI and if so, it shows
 * a login modal. After the user successfully logs in, it calls `actionToRetry` again.
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
 * cluster the login modal should use.
 */
export async function retryWithRelogin<T>(
  appContext: AppContext,
  originatingDocumentUri: string,
  resourceUri: string,
  actionToRetry: () => Promise<T>
): Promise<T> {
  let retryableErrorFromActionToRetry: Error;
  try {
    return await actionToRetry();
  } catch (error) {
    // TODO(ravicious): Replace this with actual check on metadata.
    const isRetryable =
      error instanceof Error &&
      (error.message.includes('ssh: handshake failed') ||
        error.message.includes('ssh: cert has expired'));

    if (isRetryable) {
      retryableErrorFromActionToRetry = error;
      logger.info(`Activating relogin on error ${error}`);
    } else {
      throw error;
    }
  }

  const isDocumentStillActive = appContext.workspacesService.isDocumentActive(
    originatingDocumentUri
  );

  if (!isDocumentStillActive) {
    // The error is retryable, but the user is no longer looking at the relevant UI, for example
    // they switched to a different document or a different workspace completely.
    //
    // Since it might take a few seconds before the execution gets to this point, the user might no
    // longer remember what their intent was, thus showing them a login modal can be disorienting.
    //
    // In that situation, let's just not attempt to relogin and instead let's surface the original
    // error.
    throw retryableErrorFromActionToRetry;
  }

  const rootClusterUri = routing.ensureRootClusterUri(resourceUri);

  await login(appContext, rootClusterUri);

  return await actionToRetry();
}

// Notice that we don't differentiate between onSuccess and onCancel. In both cases, we're going to
// retry the action anyway in case the cert was refreshed externally before the modal was closed,
// for example through tsh login.
function login(appContext: AppContext, rootClusterUri: string): Promise<void> {
  return new Promise(resolve => {
    appContext.modalsService.openClusterConnectDialog({
      clusterUri: rootClusterUri,
      onSuccess: () => resolve(),
      onCancel: () => resolve(),
    });
  });
}
