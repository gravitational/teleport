/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { QueryClientProvider } from '@tanstack/react-query';

import {
  render,
  screen,
  testQueryClient,
  waitFor,
  within,
} from 'design/utils/testing';
import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationAwsOidc,
  IntegrationKind,
  IntegrationWithSummary,
  integrationService,
} from 'teleport/services/integrations';

import { SettingsTab } from './SettingsTab';

const baseStats: IntegrationWithSummary = {
  name: 'my-aws-integration',
  subKind: IntegrationKind.AwsOidc,
  unresolvedUserTasks: 0,
  userTasks: [],
  awsra: {} as any,
  awsoidc: {} as any,
  awsec2: {} as any,
  awsrds: {} as any,
  awseks: {} as any,
  azurevm: {} as any,
  rolesAnywhereProfileSync: {} as any,
};

const baseIntegration: IntegrationAwsOidc = {
  resourceType: 'integration',
  kind: IntegrationKind.AwsOidc,
  name: 'my-aws-integration',
  spec: {
    roleArn: 'arn:aws:iam::123456789012:role/example',
  },
  statusCode: 1,
};

function setupMocks(integration: IntegrationAwsOidc = baseIntegration) {
  jest
    .spyOn(integrationService, 'fetchIntegration')
    .mockResolvedValue(integration);
  jest.spyOn(integrationService, 'fetchIntegrationRules').mockResolvedValue({
    rules: [],
    nextKey: '',
  });
}

function renderSettingsTab() {
  const ctx = createTeleportContext();
  ctx.storeUser.state.cluster.authVersion = '1.0.0';

  return render(
    <ContextProvider ctx={ctx}>
      <QueryClientProvider client={testQueryClient}>
        <SettingsTab
          stats={baseStats}
          activeInfoGuideTab={null}
          onInfoGuideTabChange={() => {}}
        />
      </QueryClientProvider>
    </ContextProvider>
  );
}

describe('SettingsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    testQueryClient.clear();
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  describe('organization scope', () => {
    test('does not render for single account', async () => {
      setupMocks();
      renderSettingsTab();

      await waitFor(() => {
        expect(
          screen.getByRole('radio', { name: 'Single Account' })
        ).toBeChecked();
      });
      expect(
        screen.queryByText(/Include Organizational Units/i)
      ).not.toBeInTheDocument();
    });

    test('renders with organization scope', async () => {
      setupMocks({
        ...baseIntegration,
        spec: {
          ...baseIntegration.spec,
          organization: {
            includeUnits: ['ou-1', 'ou-2'],
            excludeUnits: ['ou-3'],
          },
        },
      });
      renderSettingsTab();

      await waitFor(() => {
        expect(
          screen.getByRole('radio', { name: 'Organization' })
        ).toBeChecked();
      });

      const includeGroup = screen.getByRole('group', {
        name: /Include Organizational Units/i,
      });
      expect(
        within(includeGroup).getByDisplayValue('ou-1')
      ).toBeInTheDocument();
      expect(
        within(includeGroup).getByDisplayValue('ou-2')
      ).toBeInTheDocument();

      const excludeGroup = screen.getByRole('group', {
        name: /Exclude Organizational Units/i,
      });
      expect(
        within(excludeGroup).getByDisplayValue('ou-3')
      ).toBeInTheDocument();
    });
  });
});
