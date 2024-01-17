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

import { MainProcessClient } from 'teleterm/mainProcess/types';
import {
  Cluster,
  CreateConnectMyComputerRoleResponse,
  Server,
  TshAbortSignal,
  TshdClient,
} from 'teleterm/services/tshd/types';

import type * as uri from 'teleterm/ui/uri';

export class ConnectMyComputerService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private tshClient: TshdClient
  ) {}

  async downloadAgent(): Promise<void> {
    await this.mainProcessClient.downloadAgent();
  }

  createRole(
    rootClusterUri: uri.RootClusterUri
  ): Promise<CreateConnectMyComputerRoleResponse> {
    return this.tshClient.createConnectMyComputerRole(rootClusterUri);
  }

  async createAgentConfigFile(rootCluster: Cluster): Promise<{
    token: string;
  }> {
    const { token } = await this.tshClient.createConnectMyComputerNodeToken(
      rootCluster.uri
    );

    await this.mainProcessClient.createAgentConfigFile({
      rootClusterUri: rootCluster.uri,
      proxy: rootCluster.proxyHost,
      token: token,
      username: rootCluster.loggedInUser.name,
    });

    return { token };
  }

  runAgent(rootClusterUri: uri.RootClusterUri): Promise<void> {
    return this.mainProcessClient.runAgent({
      rootClusterUri,
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
