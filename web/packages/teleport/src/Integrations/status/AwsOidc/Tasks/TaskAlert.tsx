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
import { useHistory } from 'react-router';

import { Alert } from 'design';
import { ArrowForward, BellRinging } from 'design/Icon';

import cfg from 'teleport/config';
import { AwsResource } from 'teleport/Integrations/status/AwsOidc/Cards/StatCard';
import { IntegrationKind } from 'teleport/services/integrations';

export function TaskAlert({
  name,
  pendingTasksCount,
  kind = IntegrationKind.AwsOidc,
  taskType,
}: {
  name: string;
  pendingTasksCount: number;
  kind?: IntegrationKind;
  taskType?: AwsResource;
}) {
  const history = useHistory();
  if (pendingTasksCount == 0) {
    return null;
  }

  return (
    <Alert
      kind="warning"
      icon={BellRinging}
      mb={0}
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
      {pendingTasksCount} pending tasks
      {taskType && ` are affecting ${taskType.toUpperCase()} rule`}
    </Alert>
  );
}
