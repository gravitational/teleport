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

import type * as types from 'teleterm/services/tshd/types';
import type * as uri from 'teleterm/ui/uri';

export class ResourcesService {
  constructor(private tshClient: types.TshClient) {}

  fetchServers(params: types.GetResourcesParams) {
    return this.tshClient.getServers(params);
  }

  // TODO(ravicious): Refactor it to use logic similar to that in the Web UI.
  // https://github.com/gravitational/teleport/blob/2a2b08dbfdaf71706a5af3812d3a7ec843d099b4/lib/web/apiserver.go#L2471
  async getServerByHostname(
    clusterUri: uri.ClusterUri,
    hostname: string
  ): Promise<types.Server | undefined> {
    const query = `name == "${hostname}"`;
    const { agentsList: servers } = await this.fetchServers({
      clusterUri,
      query,
      limit: 2,
      sort: null,
    });

    if (servers.length > 1) {
      throw new AmbiguousHostnameError(hostname);
    }

    return servers[0];
  }

  fetchDatabases(params: types.GetResourcesParams) {
    return this.tshClient.getDatabases(params);
  }

  fetchKubes(params: types.GetResourcesParams) {
    return this.tshClient.getKubes(params);
  }
}

export class AmbiguousHostnameError extends Error {
  constructor(hostname: string) {
    super(`Ambiguous hostname "${hostname}"`);
    this.name = 'AmbiguousHostname';
  }
}
