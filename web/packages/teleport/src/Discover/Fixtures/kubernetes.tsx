/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { PropsWithChildren } from 'react';

import { TeleportProvider } from 'teleport/Discover/Fixtures/fixtures';
import { KubeLocation, ResourceSpec } from 'teleport/Discover/SelectResource';
import { KUBERNETES } from 'teleport/Discover/SelectResource/resources';
import { ResourceKind } from 'teleport/Discover/Shared';
import { EksMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

export function getKubeResourceSpec(location?: KubeLocation): ResourceSpec {
  return {
    ...KUBERNETES[1],
    kubeMeta: {
      location,
    },
  };
}

export function getEksMeta(): EksMeta {
  return {
    resourceName: 'eks1',
    awsRegion: 'us-east-1',
    agentMatcherLabels: [],
    kube: {
      kind: 'kube_cluster',
      name: 'eks1',
      labels: [],
    },
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'test-integration',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn:aws:iam::123456789012:role/test-role-arn',
        issuerS3Bucket: '',
        issuerS3Prefix: '',
      },
      statusCode: IntegrationStatusCode.Running,
    },
  };
}

export const ComponentWrapper: React.FC<PropsWithChildren> = ({ children }) => (
  <TeleportProvider
    agentMeta={getEksMeta()}
    resourceKind={ResourceKind.Kubernetes}
    resourceSpec={getKubeResourceSpec(KubeLocation.Aws)}
  >
    {children}
  </TeleportProvider>
);
