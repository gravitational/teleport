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

import { MainProcessClient } from 'teleterm/mainProcess/types';
import {
  Cluster,
  CreateConnectMyComputerRoleResponse,
  Server,
  TshAbortSignal,
  TshClient,
} from 'teleterm/services/tshd/types';

import type * as uri from 'teleterm/ui/uri';

export class ConnectMyComputerService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private tshClient: TshClient
  ) {}

  async downloadAgent(): Promise<void> {
    await this.mainProcessClient.downloadAgent();
  }

  createRole(
    rootClusterUri: uri.RootClusterUri
  ): Promise<CreateConnectMyComputerRoleResponse> {
    return this.tshClient.createConnectMyComputerRole(rootClusterUri);
  }

  async createToken(rootClusterUri: uri.RootClusterUri): Promise<{
    token: string;
  }> {
    return this.tshClient.createConnectMyComputerNodeToken(rootClusterUri);
  }

  runAgent(rootCluster: Cluster, token: string): Promise<void> {
    return this.mainProcessClient.runAgent({
      rootClusterUri: rootCluster.uri,
      proxy: rootCluster.proxyHost,
      username: rootCluster.loggedInUser.name,
      token,
    });
  }

  killAgent(rootClusterUri: uri.RootClusterUri): Promise<void> {
    return this.mainProcessClient.killAgent({ rootClusterUri });
  }

  isAgentConfigFileCreated(
    rootClusterUri: uri.RootClusterUri
  ): Promise<boolean> {
    return this.mainProcessClient.isAgentConfigFileCreated({ rootClusterUri });
  }

  // TODO(ravicious): Remove this.
  // https://github.com/gravitational/teleport/issues/34531
  deleteToken(
    rootClusterUri: uri.RootClusterUri,
    token: string
  ): Promise<void> {
    return this.tshClient.deleteConnectMyComputerToken(rootClusterUri, token);
  }

  removeConnectMyComputerNode(
    rootClusterUri: uri.RootClusterUri
  ): Promise<void> {
    return this.tshClient.deleteConnectMyComputerNode(rootClusterUri);
  }

  removeAgentDirectory(rootClusterUri: uri.RootClusterUri): Promise<void> {
    return this.mainProcessClient.removeAgentDirectory({ rootClusterUri });
  }

  getConnectMyComputerNodeName(
    rootClusterUri: uri.RootClusterUri
  ): Promise<string> {
    return this.tshClient.getConnectMyComputerNodeName(rootClusterUri);
  }

  async killAgentAndRemoveData(
    rootClusterUri: uri.RootClusterUri
  ): Promise<void> {
    await this.killAgent(rootClusterUri);
    await this.mainProcessClient.removeAgentDirectory({ rootClusterUri });
  }

  async waitForNodeToJoin(
    rootClusterUri: uri.RootClusterUri,
    abortSignal: TshAbortSignal
  ): Promise<Server> {
    const response = await this.tshClient.waitForConnectMyComputerNodeJoin(
      rootClusterUri,
      abortSignal
    );

    return response.server;
  }
}
