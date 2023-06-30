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

import { Cluster } from 'teleterm/services/tshd/types';
import { ConfigService } from 'teleterm/services/config';
import { RuntimeSettings } from 'teleterm/mainProcess/types';

/**
 * Checks if Connect My Computer can be used for the given root cluster.
 *
 * The root cluster is required because `loggedInUser` and `features` are not fully defined for leaves.
 * */
export function canUseConnectMyComputer(
  rootCluster: Cluster,
  configService: ConfigService,
  runtimeSettings: RuntimeSettings
): boolean {
  if (rootCluster.leaf) {
    return false;
  }

  const isUnix =
    runtimeSettings.platform === 'darwin' ||
    runtimeSettings.platform === 'linux';

  return (
    isUnix &&
    rootCluster.loggedInUser?.acl?.tokens.create &&
    rootCluster.features?.isUsageBasedBilling &&
    configService.get('feature.connectMyComputer').value
  );
}
