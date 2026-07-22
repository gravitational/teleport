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

import React from 'react';

import cfg from 'teleport/config';
import { CreateDatabase } from 'teleport/Discover/Database/CreateDatabase';
import { DatabaseWrapper } from 'teleport/Discover/Database/DatabaseWrapper';
import { DeployService } from 'teleport/Discover/Database/DeployService';
import { ManualDeploy } from 'teleport/Discover/Database/DeployService/ManualDeploy';
import { EnrollRdsDatabase } from 'teleport/Discover/Database/EnrollRdsDatabase';
import { IamPolicy } from 'teleport/Discover/Database/IamPolicy';
import { MutualTls } from 'teleport/Discover/Database/MutualTls';
import { SetupAccess } from 'teleport/Discover/Database/SetupAccess';
import { TestConnection } from 'teleport/Discover/Database/TestConnection';
import { ResourceViewConfig } from 'teleport/Discover/flow';
import {
  DatabaseLocation,
  ResourceSpec,
} from 'teleport/Discover/SelectResource';
import { AwsAccount, Finished, ResourceKind } from 'teleport/Discover/Shared';
import { DiscoverEvent } from 'teleport/services/userEvent';

import { ConfigureDiscoveryService } from '../Shared/ConfigureDiscoveryService';

export const DatabaseResource: ResourceViewConfig<ResourceSpec> = {
  kind: ResourceKind.Database,
  wrapper(component: React.ReactNode) {
    return <DatabaseWrapper>{component}</DatabaseWrapper>;
  },
  shouldPrompt(currentStep, currentView, resourceSpec) {
    if (resourceSpec.dbMeta?.location === DatabaseLocation.Aws) {
      // Allow user to bypass prompting on this step (Connect AWS Connect)
      // on exit because users might need to change route to setup an
      // integration.
      if (currentStep === 0) {
        return false;
      }
    }
    return currentView?.eventName !== DiscoverEvent.Completed;
  },
  views(resource) {
    let configureResourceViews;
    if (resource && resource.dbMeta) {
      switch (resource.dbMeta.location) {
        case DatabaseLocation.Aws:
          configureResourceViews = [
            {
              title: 'Connect AWS Account',
              component: AwsAccount,
              eventName: DiscoverEvent.IntegrationAWSOIDCConnectEvent,
            },
            {
              title: 'Enroll RDS Database',
              component: EnrollRdsDatabase,
              eventName: DiscoverEvent.DatabaseRDSEnrollEvent,
            },
            // Self hosted requires user to manually install a discovery service
            // for auto discovery.
            // Cloud already has a discovery service running, so this step is not required.
            ...(!cfg.isCloud
              ? [
                  {
                    title: 'Configure Discovery Service',
                    component: () => (
                      <ConfigureDiscoveryService withCreateConfig={true} />
                    ),
                    eventName: DiscoverEvent.CreateDiscoveryConfig,
                  },
                ]
              : []),
            // There are two types of deploy service methods:
            //  - manual: user deploys it whereever they want OR
            //  - auto (default): we deploy for them using aws
            //    fargate container
            {
              title: 'Deploy Database Service',
              component: DeployService,
              eventName: DiscoverEvent.DeployService,
              manuallyEmitSuccessEvent: true,
            },
            // This step can be skipped for the following.
            // In the enroll RDS step:
            //  - if user opted to auto-enroll all databases
            //  - or if a db service was already found and in the db server
            //    polling result there is a iamPolicyStatus === Success
            // Or if user auto deployed a database service (the first step
            // requires them to configure IAM policy)
            {
              title: 'Configure IAM Policy',
              component: IamPolicy,
              eventName: DiscoverEvent.DatabaseConfigureIAMPolicy,
            },
          ];

          break;

        case DatabaseLocation.SelfHosted:
          configureResourceViews = [
            {
              title: 'Register a Database',
              component: CreateDatabase,
              eventName: DiscoverEvent.DatabaseRegister,
            },
            {
              title: 'Deploy Database Service',
              component: ManualDeploy,
              eventName: DiscoverEvent.DeployService,
            },
            {
              title: 'Configure mTLS',
              component: MutualTls,
              eventName: DiscoverEvent.DatabaseConfigureMTLS,
            },
          ];

          break;
      }
    }

    return [
      {
        title: 'Configure Resource',
        views: configureResourceViews,
      },
      {
        title: 'Set Up Access',
        component: SetupAccess,
        eventName: DiscoverEvent.PrincipalsConfigure,
      },
      {
        title: 'Test Connection',
        component: TestConnection,
        eventName: DiscoverEvent.TestConnection,
        manuallyEmitSuccessEvent: true,
      },
      {
        title: 'Finished',
        component: Finished,
        eventName: DiscoverEvent.Completed,
        hide: true,
      },
    ];
  },
};
