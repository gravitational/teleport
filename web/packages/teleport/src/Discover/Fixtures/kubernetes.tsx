/**
 * Copyright 2024 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { PropsWithChildren } from 'react';

import { TeleportProvider } from 'teleport/Discover/Fixtures/fixtures';
import { ResourceKind } from 'teleport/Discover/Shared';
import { KubeLocation, ResourceSpec } from 'teleport/Discover/SelectResource';
import { EksMeta } from 'teleport/Discover/useDiscover';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { KUBERNETES } from 'teleport/Discover/SelectResource/resources';

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
