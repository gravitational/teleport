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
import { useHistory } from 'react-router';
import styled, { useTheme } from 'styled-components';

import { Flex, Indicator } from 'design';
import { Danger } from 'design/Alert';
import Table, { Cell } from 'design/DataTable';
import { Notification, NotificationItem } from 'shared/components/Notification';

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
  const theme = useTheme();
  const history = useHistory();
  const searchParams = new URLSearchParams(history.location.search);
  const [notification, setNotification] = useState<NotificationItem>();

  const { integrationAttempt } = useAwsOidcStatus();
  const { data: integration } = integrationAttempt;
  const [selectedTask, setSelectedTask] = useState<string>('');

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

  // use updated query params to set/unset the task side panel
  useEffect(() => {
    const taskName = searchParams.get('task');
    if (
      taskName &&
      taskName !== '' &&
      serverSidePagination.fetchedData.agents
    ) {
      setSelectedTask(taskName);
    } else {
      setSelectedTask('');
    }
  }, [searchParams, serverSidePagination?.fetchedData]);

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

  function closeTask(resolved: boolean) {
    if (resolved) {
      // If there are multiple pages, we would rather refresh the table with X results rather than
      // use modifyFetchedData to remove the item.
      serverSidePagination.fetch();

      setNotification({
        content: {
          description:
            'The task has been marked as resolved; it will reappear in the table if the issue persists after the next sync.',
        },
        severity: 'success',
        id: selectedTask,
      });
    }
    history.replace(history.location.pathname);
  }

  function openTask(task: UserTask) {
    const urlParams = new URLSearchParams();
    urlParams.append('task', task.name);
    history.replace(`${history.location.pathname}?${urlParams.toString()}`);
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
                openTask(row);
              },
              getStyle: (row: UserTask) => {
                if (row.name === selectedTask) {
                  return {
                    backgroundColor: theme.colors.interactive.tonal.primary[0],
                  };
                }
                return { cursor: 'pointer' };
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
                key: 'title',
                headerText: 'Issue Details',
              },
              {
                key: 'lastStateChange',
                headerText: 'Timestamp (UTC)',
                render: item => (
                  <Cell>{new Date(item.lastStateChange).toISOString()}</Cell>
                ),
              },
            ]}
            emptyText={`No pending tasks`}
            pagination={{
              pageSize: serverSidePagination.pageSize,
              pagerPosition: 'both',
            }}
            fetching={{
              fetchStatus: serverSidePagination.fetchStatus,
              onFetchNext: serverSidePagination.fetchNext,
              onFetchPrev: serverSidePagination.fetchPrev,
            }}
          />
          {notification && (
            <NotificationContainer>
              <Notification
                key={notification.id}
                item={notification}
                onRemove={() => setNotification(undefined)}
                minWidth="432px"
              />
            </NotificationContainer>
          )}
        </FeatureBox>
      </Flex>
      {selectedTask && <Task name={selectedTask} close={closeTask} />}
    </Flex>
  );
}

const NotificationContainer = styled.div`
  position: absolute;
  top: ${props => props.theme.space[10]}px;
  right: ${props => props.theme.space[5]}px;
`;
