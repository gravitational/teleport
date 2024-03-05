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

import { ITerminalServiceClient } from 'gen-proto-ts/teleport/lib/teleterm/v1/service_pb.client';

import { MainProcessClient } from 'teleterm/mainProcess/types';
import {
  Cluster,
  CreateConnectMyComputerRoleResponse,
  Server,
} from 'teleterm/services/tshd/types';
import { ObjectifiedAbortSignal } from 'teleterm/services/tshd/grpcContextBridgeClient';

import type * as uri from 'teleterm/ui/uri';

export class ConnectMyComputerService {
  constructor(
    private mainProcessClient: MainProcessClient,
    private terminalServiceClient: ITerminalServiceClient
  ) {}

  async downloadAgent(): Promise<void> {
    await this.mainProcessClient.downloadAgent();
  }

  async verifyAgent(): Promise<void> {
    await this.mainProcessClient.verifyAgent();
  }

  createRole(
    rootClusterUri: uri.RootClusterUri
  ): Promise<CreateConnectMyComputerRoleResponse> {
    return this.terminalServiceClient.createConnectMyComputerRole({
      rootClusterUri,
    }).response;
  }

  async createAgentConfigFile(rootCluster: Cluster): Promise<void> {
    const { token } =
      await this.terminalServiceClient.createConnectMyComputerNodeToken({
        rootClusterUri: rootCluster.uri,
      }).response;

    await this.mainProcessClient.createAgentConfigFile({
      rootClusterUri: rootCluster.uri,
      proxy: rootCluster.proxyHost,
      token: token,
      username: rootCluster.loggedInUser.name,
    });
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

  async removeConnectMyComputerNode(
    rootClusterUri: uri.RootClusterUri
  ): Promise<void> {
    await this.terminalServiceClient.deleteConnectMyComputerNode({
      rootClusterUri,
    }).response;
  }

  removeAgentDirectory(rootClusterUri: uri.RootClusterUri): Promise<void> {
    return this.mainProcessClient.removeAgentDirectory({ rootClusterUri });
  }

  async getConnectMyComputerNodeName(
    rootClusterUri: uri.RootClusterUri
  ): Promise<string> {
    const res = await this.terminalServiceClient.getConnectMyComputerNodeName({
      rootClusterUri,
    }).response;
    return res.name;
  }

  async killAgentAndRemoveData(
    rootClusterUri: uri.RootClusterUri
  ): Promise<void> {
    await this.killAgent(rootClusterUri);
    await this.mainProcessClient.removeAgentDirectory({ rootClusterUri });
  }

  async waitForNodeToJoin(
    rootClusterUri: uri.RootClusterUri,
    abortSignal: ObjectifiedAbortSignal
  ): Promise<Server> {
    const response =
      await this.terminalServiceClient.waitForConnectMyComputerNodeJoin(
        {
          rootClusterUri,
        },
        { abort: abortSignal }
      ).response;

    return response.server as Server;
  }
}
