/**
 * Copyright 2023 Gravitational, Inc.
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
import { MemoryRouter } from 'react-router';

import {
  DatabaseEngine,
  DatabaseLocation,
  ResourceSpec,
} from 'teleport/Discover/SelectResource';
import {
  IntegrationKind,
  IntegrationStatusCode,
} from 'teleport/services/integrations';
import { ContextProvider } from 'teleport';
import {
  DiscoverProvider,
  DiscoverContextState,
  AgentMeta,
  DbMeta,
} from 'teleport/Discover/useDiscover';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { PingTeleportProvider } from 'teleport/Discover/Shared/PingTeleportContext';
import { FeaturesContextProvider } from 'teleport/FeaturesContext';
import { ResourceKind } from 'teleport/Discover/Shared';
import cfg from 'teleport/config';
import { IamPolicyStatus } from 'teleport/services/databases';
import { Acl, AuthType } from 'teleport/services/user';

import { DATABASES } from './SelectResource/databases';

export const TeleportProvider: React.FC<
  PropsWithChildren<{
    agentMeta: AgentMeta;
    resourceSpec?: ResourceSpec;
    interval?: number;
    customAcl?: Acl;
    authType?: AuthType;
  }>
> = props => {
  const ctx = createTeleportContext({ customAcl: props.customAcl });
  if (props.authType) {
    ctx.storeUser.state.authType = props.authType;
  }
  const discoverCtx: DiscoverContextState = {
    agentMeta: props.agentMeta,
    exitFlow: () => null,
    viewConfig: null,
    indexedViews: [],
    setResourceSpec: () => null,
    updateAgentMeta: () => null,
    emitErrorEvent: () => null,
    emitEvent: () => null,
    eventState: null,
    currentStep: 0,
    nextStep: () => null,
    prevStep: () => null,
    onSelectResource: () => null,
    handleAndEmitRequestError: () => null,
    resourceSpec: props.resourceSpec
      ? props.resourceSpec
      : getDbResourceSpec(DatabaseEngine.Postgres, DatabaseLocation.Aws),
  };

  return (
    <MemoryRouter initialEntries={[{ pathname: cfg.routes.discover }]}>
      <ContextProvider ctx={ctx}>
        <FeaturesContextProvider value={[]}>
          <DiscoverProvider mockCtx={discoverCtx}>
            <PingTeleportProvider
              interval={props.interval || 100000}
              resourceKind={ResourceKind.Database}
            >
              {props.children}
            </PingTeleportProvider>
          </DiscoverProvider>
        </FeaturesContextProvider>
      </ContextProvider>
    </MemoryRouter>
  );
};

export function getDbResourceSpec(
  engine: DatabaseEngine,
  location?: DatabaseLocation
): ResourceSpec {
  return {
    ...DATABASES[0],
    dbMeta: {
      engine,
      location,
    },
  };
}

export function getDbMeta(): DbMeta {
  return {
    resourceName: 'db-name',
    awsRegion: 'us-east-1',
    agentMatcherLabels: [],
    db: {
      aws: {
        iamPolicyStatus: IamPolicyStatus.Unspecified,
        rds: {
          region: 'us-east-1',
          vpcId: 'test-vpc',
          resourceId: 'some-rds-resource-id',
          subnets: [],
        },
      },
      kind: 'db',
      name: 'some-db-name',
      description: 'some-description',
      type: 'rds',
      protocol: 'postgres',
      labels: [],
      hostname: 'some-db-hostname',
      names: ['dynamicName1', 'dynamicName2'],
      users: ['dynamicUser1', 'dynamicUser2'],
    },
    selectedAwsRdsDb: { region: 'us-east-1' } as any,
    awsIntegration: {
      kind: IntegrationKind.AwsOidc,
      name: 'test-integration',
      resourceType: 'integration',
      spec: {
        roleArn: 'arn-123',
      },
      statusCode: IntegrationStatusCode.Running,
    },
  };
}
