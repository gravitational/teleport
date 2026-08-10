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
import { createMemoryHistory } from 'history';
import { Router } from 'react-router';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { Route } from 'teleport/components/Router';
import cfg from 'teleport/config';
import { Tasks } from 'teleport/Integrations/status/AwsOidc/Tasks/Tasks';
import { makeAwsOidcStatusContextState } from 'teleport/Integrations/status/AwsOidc/testHelpers/makeAwsOidcStatusContextState';
import { awsOidcStatusContext } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import {
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import TeleportContext from 'teleport/teleportContext';

const integrationName = 'integration-test';

test('deep links an open task', async () => {
  const ctx = new TeleportContext();
  jest
    .spyOn(integrationService, 'fetchIntegrationUserTasksList')
    .mockResolvedValue({
      items: [
        {
          name: 'df4d8288-7106-5a50-bb50-4b5858e48ad5',
          taskType: 'discover-rds',
          state: 'OPEN',
          integration: integrationName,
          lastStateChange: '2025-02-11T20:32:19.482607921Z',
          issueType: 'rds-failure',
          title: 'RDS Failure',
        },
      ],
      nextKey: 'next',
    });

  const history = createMemoryHistory({
    initialEntries: [
      cfg.getIntegrationTasksRoute(IntegrationKind.AwsOidc, integrationName),
    ],
  });
  history.replace = jest.fn();

  render(
    <Router history={history}>
      <ContextProvider ctx={ctx}>
        <awsOidcStatusContext.Provider value={makeAwsOidcStatusContextState()}>
          <Route path={cfg.routes.integrationTasks} render={() => <Tasks />} />
        </awsOidcStatusContext.Provider>
      </ContextProvider>
    </Router>
  );

  await screen.findAllByText('Pending Tasks');
  await userEvent.click(screen.getByText('RDS Failure'));

  await waitFor(() =>
    expect(history.replace).toHaveBeenCalledWith(
      '/web/integrations/status/aws-oidc/integration-test/tasks?task=df4d8288-7106-5a50-bb50-4b5858e48ad5'
    )
  );
});
