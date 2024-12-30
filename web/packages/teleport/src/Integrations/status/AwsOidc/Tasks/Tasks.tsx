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

import Table, { Cell } from 'design/DataTable';

import { Indicator } from 'design';

import { Danger } from 'design/Alert';

import { FeatureBox } from 'teleport/components/Layout';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { UserTask } from 'teleport/services/integrations';
import { AwsOidcTitle } from 'teleport/Integrations/status/AwsOidc/AwsOidcTitle';

export function Tasks() {
  const { integrationAttempt, tasksAttempt } = useAwsOidcStatus();
  const { data: integration } = integrationAttempt;
  const { data: tasks } = tasksAttempt;

  if (
    integrationAttempt.status == 'processing' ||
    tasksAttempt.status == 'processing'
  ) {
    return <Indicator />;
  }

  if (integrationAttempt.status == 'error' || tasksAttempt.status == 'error') {
    return (
      <Danger>
        {integrationAttempt.status == 'error'
          ? integrationAttempt.statusText
          : tasksAttempt.statusText}
      </Danger>
    );
  }

  if (!tasks) {
    return null;
  }

  return (
    <>
      {integration && <AwsOidcHeader integration={integration} tasks={true} />}
      <FeatureBox css={{ maxWidth: '1400px', paddingTop: '16px', gap: '30px' }}>
        {integration && <AwsOidcTitle integration={integration} tasks={true} />}
        <Table<UserTask>
          data={tasks.items}
          columns={[
            {
              key: 'taskType',
              headerText: 'Type',
              isSortable: true,
            },
            {
              key: 'issueType',
              headerText: 'Issue Details',
              isSortable: true,
            },
            {
              key: 'lastStateChange',
              headerText: 'Timestamp (UTC)',
              isSortable: true,
              render: (item: UserTask) => (
                <Cell>{new Date(item.lastStateChange).toISOString()}</Cell>
              ),
            },
          ]}
          emptyText={`No pending tasks`}
          isSearchable
        />
      </FeatureBox>
    </>
  );
}
