/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { PropsWithChildren, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';

import { Alert, ButtonBorder, Flex, H2 } from 'design';
import { Danger } from 'design/Alert';
import Table from 'design/DataTable';
import { TableColumn } from 'design/DataTable/types';
import { H3, P2, Subtitle2 } from 'design/Text';
import { useAsync } from 'shared/hooks/useAsync';
import useAttempt from 'shared/hooks/useAttemptNext';

import { getResourceType } from 'teleport/Integrations/status/AwsOidc/helpers';
import {
  DiscoverEc2,
  DiscoverEc2Instance,
  DiscoverEks,
  DiscoverEksCluster,
  DiscoverRds,
  DiscoverRdsDatabase,
  integrationService,
  UserTaskDetail,
} from 'teleport/services/integrations';

import { AwsResource } from '../StatCard';
import { SidePanel } from './SidePanel';

export function Task({
  name,
  close,
}: {
  name: string;
  close: (resolved: boolean) => void;
}) {
  const { attempt, setAttempt } = useAttempt('');

  const [taskAttempt, fetchTask] = useAsync(() =>
    integrationService.fetchUserTask(name)
  );

  useEffect(() => {
    fetchTask();
  }, []);

  if (taskAttempt.status === 'error') {
    return (
      <SidePanel onClose={() => close(false)}>
        <Danger>{taskAttempt.statusText}</Danger>
      </SidePanel>
    );
  }

  if (!taskAttempt.data) {
    return null;
  }

  function resolve() {
    setAttempt({ status: 'processing' });
    integrationService
      .resolveUserTask(name)
      .then(() => {
        setAttempt({ status: '', statusText: '' });
        close(true);
      })
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  const impactedInstances = getImpactedInstances(taskAttempt.data);
  const { resourceType, resource, impacts } = impactedInstances;
  const table = makeImpactsTable(impactedInstances);

  return (
    <SidePanel
      onClose={() => close(false)}
      header={<H2>{taskAttempt.data.issueType}</H2>}
      footer={
        <ButtonBorder
          intent="success"
          onClick={resolve}
          disabled={attempt.status === 'processing'}
        >
          Mark as Resolved
        </ButtonBorder>
      }
      disabled={attempt.status === 'processing'}
    >
      {attempt.status === 'failed' && (
        <Alert kind="danger" details={attempt.statusText}>
          Unable to resolve task
        </Alert>
      )}
      <Attribute title="Integration Name">
        {taskAttempt.data.integration}
      </Attribute>
      <Attribute title="Resource Type">{resourceType.toUpperCase()}</Attribute>
      <Attribute title="Region">{resource.region}</Attribute>
      <H3 my={2}>Details</H3>
      <ReactMarkdown>{taskAttempt.data.description}</ReactMarkdown>
      <H3 my={2}>Impacted instances ({Object.keys(impacts).length})</H3>
      <Table
        data={table.data}
        columns={table.columns}
        emptyText={`No impacted instances`}
      />
    </SidePanel>
  );
}

type TableInstance = {
  instanceId?: string;
  name: string;
};

function makeImpactsTable(instances: ImpactedInstances): {
  columns: TableColumn<TableInstance>[];
  data: TableInstance[];
} {
  const { resourceType, impacts } = instances;
  switch (resourceType) {
    case AwsResource.ec2:
      return {
        columns: [
          {
            key: 'instanceId',
            headerText: 'Instance ID',
          },
          {
            key: 'name',
            headerText: 'Instance Name',
          },
        ],
        data: Object.keys(impacts).map(i => ({
          instanceId: impacts[i].instance_id,
          name: impacts[i].name,
        })),
      };
    case AwsResource.eks:
    case AwsResource.rds:
      return {
        columns: [
          {
            key: 'name',
            headerText: 'Name',
          },
        ],
        data: Object.keys(impacts).map(i => ({
          name: impacts[i].name,
        })),
      };
    default:
      resourceType satisfies never;
  }
}

type ImpactedInstances =
  | {
      resourceType: AwsResource.ec2;
      resource: DiscoverEc2;
      impacts: Record<string, DiscoverEc2Instance>;
    }
  | {
      resourceType: AwsResource.eks;
      resource: DiscoverEks;
      impacts: Record<string, DiscoverEksCluster>;
    }
  | {
      resourceType: AwsResource.rds;
      resource: DiscoverRds;
      impacts: Record<string, DiscoverRdsDatabase>;
    };

function getImpactedInstances(task: UserTaskDetail): ImpactedInstances {
  const resourceType = getResourceType(task.taskType);

  switch (resourceType) {
    case AwsResource.ec2:
      return {
        resourceType: resourceType,
        resource: task.discoverEc2,
        impacts: task.discoverEc2?.instances,
      };
    case AwsResource.eks:
      return {
        resourceType: resourceType,
        resource: task.discoverEks,
        impacts: task.discoverEks?.clusters,
      };
    case AwsResource.rds:
    default:
      return {
        resourceType: resourceType,
        resource: task.discoverRds,
        impacts: task.discoverRds?.databases,
      };
  }
}

const Attribute = ({
  title = '',
  children,
}: PropsWithChildren<{ title: string }>) => (
  <Flex mb={1} alignItems="center">
    <Subtitle2 style={{ minWidth: '150px' }}>{title}:</Subtitle2>
    <P2
      style={{
        whiteSpace: 'pre',
        textWrap: 'wrap',
        width: '100%',
        wordBreak: 'break-all',
        margin: 0,
      }}
      data-testid={title}
    >
      {children || `N/A`}
    </P2>
  </Flex>
);
