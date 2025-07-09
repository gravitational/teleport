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
                issueType: 'rds-auth-disabled',
                integration: integrationName,
                lastStateChange: '2022-02-12T20:32:19.482607921Z',
                // lib/usertasks/descriptions/rds-iam-auth-disabled.md
                description: `
                # IAM Auth disabled
The Teleport Database Service uses [IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html) to communicate with RDS.

The following RDS databases do not have IAM authentication enabled.

You can enable by modifying the IAM DB Authentication property of the database.
                `,
                title: 'rds-auth-disabled',
                discoverRds: {
                  databases: {
                    'i-01568fcc52b7071cd': {
                      name: 'i-01568fcc52b7071cd',
                      is_cluster: true,
                      engine: 'aurora',
                      discovery_config: 'ee2f2480-09d3-4409-80f5-183549706446',
                      discovery_group: 'cloud-discovery-group',
                      sync_time: {
                        seconds: -62135596800,
                      },
                      resourceUrl:
                        'https://console.aws.amazon.com/ec2/home?region=us-east-2#InstanceDetails:instanceId=i-01568fcc52b7071cd',
                    },
                  },
                },
              },
              {
                name: 'ec2-detail',
                taskType: 'discover-ec2',
                state: TaskState.Open,
                issueType: 'ec2-ssm-invocation-failure',
                integration: integrationName,
                lastStateChange: '2025-02-11T20:32:19.482607921Z',
                title: 'ec2-detail',
                // lib/usertasks/descriptions/ec2-ssm-invocation-failure.md
                description: `
                # SSM Invocation failed
Teleport failed to access the SSM Agent to auto enroll the instance.
Some instances failed to communicate with the AWS Systems Manager service to execute the install script.

Usually this happens when:

**Missing policies**

The IAM Role used by the integration might be missing some required permissions.
Ensure the following actions are allowed in the IAM Role used by the integration:
- \`ec2:DescribeInstances\`
- \`ssm:DescribeInstanceInformation\`
- \`ssm:GetCommandInvocation\`
- \`ssm:ListCommandInvocations\`
- \`ssm:SendCommand\`

**SSM Document is invalid**

Teleport uses an SSM Document to run an installation script.
If the document is changed or removed, it might no longer work.
`,
                discoverEc2: {
                  instances: {
                    'i-01568fcc52b7071cd': {
                      instance_id: 'i-01568fcc52b7071cd',
                      discovery_config: 'ee2f2480-09d3-4409-80f5-183549706446',
                      discovery_group: 'cloud-discovery-group',
                      sync_time: {
                        seconds: -62135596800,
                      },
                      resourceUrl:
                        'https://console.aws.amazon.com/ec2/home?region=us-east-2#InstanceDetails:instanceId=i-01568fcc52b7071cd',
                    },
                    'i-019e54b3f58bfa9fd': {
                      instance_id: 'i-019e54b3f58bfa9fd',
                      name: 'travis-test-tcp4',
                      discovery_config: 'ee2f2480-09d3-4409-80f5-183549706446',
                      discovery_group: 'cloud-discovery-group',
                      sync_time: {
                        seconds: -62135596800,
                      },
                      resourceUrl:
                        'https://console.aws.amazon.com/ec2/home?region=us-east-2#InstanceDetails:instanceId=i-019e54b3f58bfa9fd',
                    },
                  },
                },
              },
              {
                name: 'no match',
                state: TaskState.Open,
                issueType: 'side panel error',
                integration: integrationName,
                title: 'no match',
                lastStateChange: '0',
                instances: [],
              },
              {
                name: 'eks-detail',
                taskType: 'discover-eks',
                state: TaskState.Open,
                issueType: 'eks-agent-not-connecting',
                integration: integrationName,
                lastStateChange: '2025-02-11T20:32:13.61608091Z',
                title: 'eks-agent-not-connecting',
                // lib/usertasks/descriptions/eks-agent-not-connecting.md
                description: `
                # Teleport Agent not connecting
The process of automatically enrolling EKS Clusters into Teleport, starts by installing the [\`teleport-kube-agent\`](https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/) to the cluster.

If the installation is successful, the EKS Cluster will appear in your Resources list.

However, the following EKS Clusters did not automatically enrolled.
This usually happens when the installation is taking too long or there was an error preventing the HELM chart installation.

Open the Teleport Agent to get more information.
`,
                discoverEks: {
                  clusters: {
                    'i-01568fcc52b7071cd': {
                      name: 'i-01568fcc52b7071cd',
                      discovery_config: 'ee2f2480-09d3-4409-80f5-183549706446',
                      discovery_group: 'cloud-discovery-group',
                      sync_time: {
                        seconds: -62135596800,
                      },
                      resourceUrl:
                        'https://console.aws.amazon.com/ec2/home?region=us-east-2#InstanceDetails:instanceId=i-01568fcc52b7071cd',
                    },
                  },
                },
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
