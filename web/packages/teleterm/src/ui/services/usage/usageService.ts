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

import { SubmitConnectEventRequest } from 'gen-proto-ts/prehog/v1alpha/connect_pb';

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';

import { ClusterOrResourceUri, ClusterUri, routing } from 'teleterm/ui/uri';
import { Cluster } from 'teleterm/services/tshd/types';
import { TshdClient } from 'teleterm/services/tshd';
import { RuntimeSettings } from 'teleterm/mainProcess/types';
import { ConfigService } from 'teleterm/services/config';
import Logger from 'teleterm/logger';
import { staticConfig } from 'teleterm/staticConfig';
import { NotificationsService } from 'teleterm/ui/services/notifications';
import { DocumentOrigin } from 'teleterm/ui/services/workspacesService';

type PrehogEventReq = Omit<
  SubmitConnectEventRequest,
  'distinctId' | 'timestamp'
>;

export class UsageService {
  private logger = new Logger('UsageService');

  constructor(
    private tshClient: TshdClient,
    private configService: ConfigService,
    private notificationsService: NotificationsService,
    // `findCluster` function - it is a workaround that allows to use `UsageEventService` in `ClustersService`.
    // Otherwise, we would have a circular dependency.
    // TODO: accept `ClustersService` instead of a function.
    // discussion: https://github.com/gravitational/webapps/pull/1451#discussion_r1055364676
    private findCluster: (clusterUri: ClusterUri) => Cluster,
    private runtimeSettings: RuntimeSettings
  ) {}

  captureUserLogin(uri: ClusterOrResourceUri, connectorType: string): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping userLogin event`
      );
      return;
    }
    const { arch, platform, osVersion, appVersion } = this.runtimeSettings;
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'clusterLogin',
        clusterLogin: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
          connectorType,
          arch,
          os: platform,
          osVersion,
          appVersion,
        },
      },
    });
  }

  captureProtocolUse(
    uri: ClusterOrResourceUri,
    protocol: 'ssh' | 'kube' | 'db' | 'app',
    origin: DocumentOrigin
  ): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping protocolUse event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'protocolUse',
        protocolUse: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
          protocol,
          origin,
        },
      },
    });
  }

  captureAccessRequestCreate(
    uri: ClusterOrResourceUri,
    kind: 'role' | 'resource'
  ): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping accessRequestCreate event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'accessRequestCreate',
        accessRequestCreate: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
          kind,
        },
      },
    });
  }

  captureAccessRequestReview(uri: ClusterOrResourceUri): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping accessRequestReview event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'accessRequestReview',
        accessRequestReview: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
        },
      },
    });
  }

  captureAccessRequestAssumeRole(uri: ClusterOrResourceUri): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping accessRequestAssumeRole event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'accessRequestAssumeRole',
        accessRequestAssumeRole: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
        },
      },
    });
  }

  captureFileTransferRun(
    uri: ClusterOrResourceUri,
    { isUpload }: { isUpload: boolean }
  ): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping fileTransferRun event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'fileTransferRun',
        fileTransferRun: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
          isUpload,
        },
      },
    });
  }

  captureUserJobRoleUpdate(jobRole: string): void {
    this.reportNonAnonymizedEvent({
      event: {
        oneofKind: 'userJobRoleUpdate',
        userJobRoleUpdate: {
          jobRole,
        },
      },
    });
  }

  captureConnectMyComputerSetup(
    uri: ClusterOrResourceUri,
    properties: { success: true } | { success: false; failedStep: string }
  ): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping connectMyComputerSetup event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'connectMyComputerSetup',
        connectMyComputerSetup: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
          success: properties.success,
          failedStep:
            (properties.success === false && properties.failedStep) ||
            undefined,
        },
      },
    });
  }

  captureConnectMyComputerAgentStart(uri: ClusterOrResourceUri): void {
    const clusterProperties = this.getClusterProperties(uri);
    if (!clusterProperties) {
      this.logger.warn(
        `Missing cluster data for ${uri}, skipping connectMyComputerAgentStart event`
      );
      return;
    }
    this.reportEvent(clusterProperties.authClusterId, {
      event: {
        oneofKind: 'connectMyComputerAgentStart',
        connectMyComputerAgentStart: {
          clusterName: clusterProperties.clusterName,
          userName: clusterProperties.userName,
        },
      },
    });
  }

  private reportNonAnonymizedEvent(prehogEventReq: PrehogEventReq): void {
    this.reportEvent('', prehogEventReq);
  }

  private async reportEvent(
    authClusterId: string,
    prehogEventReq: PrehogEventReq
  ): Promise<void> {
    const isCollectingUsageMetricsEnabled = this.configService.get(
      'usageReporting.enabled'
    ).value;

    if (!staticConfig.prehogAddress || !isCollectingUsageMetricsEnabled) {
      return;
    }

    try {
      await this.tshClient.reportUsageEvent({
        authClusterId,
        prehogReq: {
          distinctId: this.runtimeSettings.installationId,
          timestamp: Timestamp.now(),
          ...prehogEventReq,
        },
      });
    } catch (e) {
      this.notificationsService.notifyWarning({
        title: 'Failed to report usage event',
        description: e.message,
      });
      this.logger.warn(`Failed to report usage event`, e.message);
    }
  }

  private getClusterProperties(uri: ClusterOrResourceUri) {
    const rootClusterUri = routing.ensureRootClusterUri(uri);
    const cluster = this.findCluster(rootClusterUri);
    if (!(cluster && cluster.loggedInUser && cluster.authClusterId)) {
      return;
    }

    return {
      authClusterId: cluster.authClusterId,
      clusterName: cluster.name,
      userName: cluster.loggedInUser.name,
    };
  }
}
