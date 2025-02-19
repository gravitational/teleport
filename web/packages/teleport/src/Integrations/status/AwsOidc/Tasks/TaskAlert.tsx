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
import { useEffect } from 'react';
import { useHistory } from 'react-router';

import { Alert } from 'design';
import { ArrowForward, BellRinging } from 'design/Icon';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import { TaskState } from 'teleport/Integrations/status/AwsOidc/Tasks/constants';
import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';

export function TaskAlert({
  name,
  kind = IntegrationKind.AwsOidc,
}: {
  name: string;
  kind?: IntegrationKind;
}) {
  const history = useHistory();
  // todo (michellescripts) should we show the banner if there is an error
  const [tasksAttempt, fetchTasks] = useAsync(() =>
    integrationService.fetchIntegrationUserTasksList(name, TaskState.Open)
  );

  useEffect(() => {
    fetchTasks();
  }, []);

  const pendingTasksCount =
    (tasksAttempt.status === 'success' &&
      tasksAttempt.data.items?.filter(t => t.state === TaskState.Open)
        .length) ||
    0;

  if (!pendingTasksCount) {
    return null;
  }

  return (
    <Alert
      kind="warning"
      icon={BellRinging}
      primaryAction={{
        content: (
          <>
            Resolve Now
            <ArrowForward size={18} ml={2} />
          </>
        ),
        onClick: () => history.push(cfg.getIntegrationTasksRoute(kind, name)),
      }}
    >
      {pendingTasksCount} Pending Tasks
    </Alert>
  );
}
