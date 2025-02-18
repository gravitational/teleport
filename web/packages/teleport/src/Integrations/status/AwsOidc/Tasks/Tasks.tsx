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

import { useEffect, useState } from 'react';

import { ButtonBorder, Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';

import { useServerSidePagination } from 'teleport/components/hooks';
import { FeatureBox } from 'teleport/components/Layout';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { AwsOidcTitle } from 'teleport/Integrations/status/AwsOidc/AwsOidcTitle';
import { getResourceType } from 'teleport/Integrations/status/AwsOidc/helpers';
import { TaskState } from 'teleport/Integrations/status/AwsOidc/Tasks/constants';
import { Task } from 'teleport/Integrations/status/AwsOidc/Tasks/Task';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { integrationService, UserTask } from 'teleport/services/integrations';

export function Tasks() {
  const { integrationAttempt } = useAwsOidcStatus();
  const { data: integration } = integrationAttempt;
  const [showTask, setShowTask] = useState<UserTask>(undefined);

  const serverSidePagination = useServerSidePagination<UserTask>({
    pageSize: 20,
    fetchFunc: async () => {
      const { items, nextKey } =
        await integrationService.fetchIntegrationUserTasksList(
          integration.name,
          TaskState.Open
        );
      return { agents: items, nextKey };
    },
    clusterId: '',
    params: {},
  });

  useEffect(() => {
    serverSidePagination.fetch();
  }, [integration]);

  if (integrationAttempt.status === 'processing') {
    return <Indicator />;
  }

  if (serverSidePagination.attempt.status === 'processing') {
    return <Indicator />;
  }

  if (integrationAttempt.status === 'error') {
    return <Danger>{integrationAttempt.statusText}</Danger>;
  }

  if (serverSidePagination.attempt.status === 'failed') {
    return <Danger>{serverSidePagination.attempt.statusText}</Danger>;
  }

  function close(resolved: boolean) {
    if (resolved) {
      // If there are multiple pages, we would rather refresh the table with X results rather than
      // use modifyFetchedData to remove the item.
      serverSidePagination.fetch();
    }
    setShowTask(undefined);
  }

  return (
    <Flex>
      <Flex
        flexDirection="column"
        css={`
          flex-grow: 1;
        `}
      >
        {integration && (
          <AwsOidcHeader integration={integration} tasks={true} />
        )}
        <FeatureBox maxWidth={1440} margin="auto" gap={3} paddingLeft={5}>
          {integration && (
            <AwsOidcTitle integration={integration} tasks={true} />
          )}
          <Table<UserTask>
            data={serverSidePagination.fetchedData?.agents || []}
            row={{
              onClick: row => {
                if (showTask === undefined) {
                  setShowTask(row);
                }
              },
              getStyle: () => {
                if (showTask === undefined) {
                  return { cursor: 'pointer' };
                }
              },
            }}
            columns={[
              {
                key: 'taskType',
                headerText: 'Type',
                render: item => (
                  <Cell>{getResourceType(item.taskType).toUpperCase()}</Cell>
                ),
              },
              {
                key: 'issueType',
                headerText: 'Issue Details',
              },
              {
                key: 'lastStateChange',
                headerText: 'Timestamp (UTC)',
                render: item => (
                  <Cell>{new Date(item.lastStateChange).toISOString()}</Cell>
                ),
              },
              {
                altKey: 'action',
                headerText: 'Actions',
                render: item => (
                  <Cell>
                    <ButtonBorder
                      onClick={() => setShowTask(item)}
                      disabled={showTask != undefined}
                      size="small"
                    >
                      View
                    </ButtonBorder>
                  </Cell>
                ),
              },
            ]}
            emptyText={`No pending tasks`}
            pagination={{ pageSize: serverSidePagination.pageSize }}
            fetching={{
              fetchStatus: serverSidePagination.fetchStatus,
              onFetchNext: serverSidePagination.fetchNext,
              onFetchPrev: serverSidePagination.fetchPrev,
            }}
          />
        </FeatureBox>
      </Flex>
      {showTask && <Task name={showTask.name} close={close} />}
    </Flex>
  );
}
