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

import { useParams } from 'react-router';

import { Danger } from 'design/Alert';

import { FeatureBox } from 'teleport/components/Layout';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { AwsOidcTitle } from 'teleport/Integrations/status/AwsOidc/AwsOidcTitle';
import { Rds } from 'teleport/Integrations/status/AwsOidc/Details/Rds';
import { Rules } from 'teleport/Integrations/status/AwsOidc/Details/Rules';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/StatCard';
import { TaskAlert } from 'teleport/Integrations/status/AwsOidc/Tasks/TaskAlert';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { IntegrationKind } from 'teleport/services/integrations';

export function Details() {
  const { resourceKind } = useParams<{
    type: IntegrationKind;
    name: string;
    resourceKind: AwsResource;
  }>();

  const { integrationAttempt, statsAttempt } = useAwsOidcStatus();

  if (integrationAttempt.status === 'error') {
    return <Danger>{integrationAttempt.statusText}</Danger>;
  }

  if (statsAttempt.status === 'error') {
    return <Danger>{statsAttempt.statusText}</Danger>;
  }

  if (!statsAttempt.data || !integrationAttempt.data) {
    return null;
  }

  const { data: integration } = integrationAttempt;
  const { awsec2, awsrds, awseks, unresolvedUserTasks } = statsAttempt.data;

  let pendingTasks = unresolvedUserTasks;
  switch (resourceKind) {
    case AwsResource.rds:
      pendingTasks = awsrds.unresolvedUserTasks;
      break;
    case AwsResource.ec2:
      pendingTasks = awsec2.unresolvedUserTasks;
      break;
    case AwsResource.eks:
      pendingTasks = awseks.unresolvedUserTasks;
      break;
  }

  return (
    <>
      {integration && (
        <AwsOidcHeader integration={integration} resource={resourceKind} />
      )}
      <FeatureBox maxWidth={1440} margin="auto" gap={3} paddingLeft={5}>
        <>
          {integration && (
            <>
              <AwsOidcTitle integration={integration} resource={resourceKind} />
              <TaskAlert
                name={integration.name}
                pendingTasksCount={pendingTasks}
                taskType={resourceKind}
              />
            </>
          )}
        </>
        {resourceKind === AwsResource.rds ? <Rds /> : <Rules />}
      </FeatureBox>
    </>
  );
}
