/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { http, HttpResponse } from 'msw';

import cfg from 'teleport/config';
import { TaskState } from 'teleport/Integrations/status/AwsOidc/Tasks/constants';
import { MockAwsOidcStatusProvider } from 'teleport/Integrations/status/AwsOidc/testHelpers/mockAwsOidcStatusProvider';
import { IntegrationKind } from 'teleport/services/integrations';

import { makeAwsOidcStatusContextState } from '../testHelpers/makeAwsOidcStatusContextState';
import { Tasks } from './Tasks';

export default {
  title: 'Teleport/Integrations/AwsOidc/Tasks',
};

const integrationName = 'integration-story';

// Empty tasks table
export function TasksEmpty() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[
        cfg.getIntegrationTasksRoute(IntegrationKind.AwsOidc, integrationName),
      ]}
      path={cfg.routes.integrationTasks}
    >
      <Tasks />
    </MockAwsOidcStatusProvider>
  );
}

// Populated tasks table
export function TaskView() {
  return (
    <MockAwsOidcStatusProvider
      value={makeAwsOidcStatusContextState()}
      initialEntries={[
        cfg.getIntegrationTasksRoute(IntegrationKind.AwsOidc, integrationName),
      ]}
      path={cfg.routes.integrationTasks}
    >
      <Tasks />
    </MockAwsOidcStatusProvider>
  );
}

TaskView.parameters = {
  msw: {
    handlers: [
      http.get(
        cfg.getIntegrationUserTasksListUrl(integrationName, TaskState.Open),
        () => {
          return HttpResponse.json({
            items: [
              {
                name: 'rds-detail',
                taskType: 'discover-rds',
                state: TaskState.Open,
                issueType: 'rds-generic',
                integration: integrationName,
                lastStateChange: '2022-02-12T20:32:19.482607921Z',
              },
              {
                name: 'ec2-detail',
                taskType: 'discover-ec2',
                state: TaskState.Open,
                issueType: 'ec2-ssm-invocation-failure',
                integration: integrationName,
                lastStateChange: '2025-02-11T20:32:19.482607921Z',
              },
              {
                name: 'ec2-detail',
                taskType: 'discover-ec2',
                state: TaskState.Open,
                issueType: 'ec2-ssm-agent-not-registered',
                integration: integrationName,
                lastStateChange: '2025-02-11T20:32:13.61608091Z',
              },
              {
                name: 'no match',
                state: TaskState.Open,
                issueType: 'side panel error',
                integration: integrationName,
                lastStateChange: '0',
              },
              {
                name: 'eks-detail',
                taskType: 'discover-eks',
                state: TaskState.Open,
                issueType: 'eks-failure',
                integration: integrationName,
                lastStateChange: '2025-02-11T20:32:13.61608091Z',
              },
            ],
            nextKey: '1',
          });
        }
      ),
      http.get(cfg.getUserTaskUrl('ec2-detail'), () => {
        return HttpResponse.json(ec2Detail);
      }),
      http.get(cfg.getUserTaskUrl('rds-detail'), () => {
        return HttpResponse.json(rdsDetail);
      }),
      http.get(cfg.getUserTaskUrl('eks-detail'), () => {
        return HttpResponse.json(eksDetail);
      }),
    ],
  },
};

const ec2Detail = {
  name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
  taskType: 'discover-ec2',
  state: 'OPEN',
  integration: integrationName,
  lastStateChange: '2025-02-11T20:32:19.482607921Z',
  issueType: 'ec2-ssm-invocation-failure',
  title: 'EC2 failure',
  description:
    'Teleport failed to access the SSM Agent to auto enroll the instance.\nSome instances failed to communicate with the AWS Systems Manager service to execute the install script.\n\nUsually this happens when:\n\n**Missing policies**\n\nThe IAM Role used by the integration might be missing some required permissions.\nEnsure the following actions are allowed in the IAM Role used by the integration:\n- `ec2:DescribeInstances`\n- `ssm:DescribeInstanceInformation`\n- `ssm:GetCommandInvocation`\n- `ssm:ListCommandInvocations`\n- `ssm:SendCommand`\n\n**SSM Document is invalid**\n\nTeleport uses an SSM Document to run an installation script.\nIf the document is changed or removed, it might no longer work.',
  discoverEc2: {
    region: 'us-east-2',
    instances: {
      'i-016e32a5882f5ee81': {
        instance_id: 'i-016e32a5882f5ee81',
      },
      'i-065818031835365cc': {
        instance_id: 'i-065818031835365cc',
        name: 'aws-test',
      },
    },
  },
};

const rdsDetail = {
  name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
  taskType: 'discover-rds',
  state: 'OPEN',
  integration: integrationName,
  lastStateChange: '2025-02-11T20:32:19.482607921Z',
  issueType: 'rds-failure',
  title: 'RDS Failure',
  description:
    'The Teleport Database Service uses [IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html) to communicate with RDS.\n',
  discoverRds: {
    databases: {
      'i-016e32a5882f5ee81': {
        name: 'i-016e32a5882f5ee81',
      },
      'i-065818031835365cc': {
        name: 'i-065818031835365cc',
      },
    },
  },
};

const eksDetail = {
  name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
  taskType: 'discover-eks',
  state: 'OPEN',
  integration: integrationName,
  lastStateChange: '2025-02-11T20:32:19.482607921Z',
  issueType: 'eks-failure',
  title: 'EKS failure',
  description:
    'Only EKS Clusters whose status is active can be automatically enrolled into teleport.\n',
  discoverEks: {
    clusters: {
      'i-016e32a5882f5ee81': {
        name: 'i-016e32a5882f5ee81',
      },
      'i-065818031835365cc': {
        name: 'i-065818031835365cc',
      },
    },
  },
};
