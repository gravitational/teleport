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

import { render, screen, testQueryClient, waitFor } from 'design/utils/testing';
import 'shared/components/TextEditor/TextEditor.mock';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import {
  IntegrationAzureOidc,
  IntegrationDiscoveryRule,
  IntegrationKind,
  IntegrationWithSummary,
  integrationService,
} from 'teleport/services/integrations';

import { SettingsTab } from './SettingsTab';

const baseStats: IntegrationWithSummary = {
  name: 'my-azure-integration',
  subKind: IntegrationKind.AzureOidc,
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

const baseIntegration: IntegrationAzureOidc = {
  resourceType: 'integration',
  kind: IntegrationKind.AzureOidc,
  name: 'my-azure-integration',
  spec: {
    tenantId: 'tenant-123',
    clientId: 'client-456',
    managedIdentity: {
      resourceGroup: 'my-rg',
      region: 'eastus',
      managementGroupId: '',
    },
  },
  statusCode: 1,
};

const baseRule: IntegrationDiscoveryRule = {
  resourceType: 'vm',
  region: 'eastus',
  labelMatcher: [],
  subscriptions: ['sub-a'],
  resourceGroups: [],
  discoveryConfig: 'dc-1',
  lastSync: 0,
};

function setupMocks(rules: IntegrationDiscoveryRule[] = []) {
  jest
    .spyOn(integrationService, 'fetchIntegration')
    .mockResolvedValue(baseIntegration);
  jest
    .spyOn(integrationService, 'fetchIntegrationRules')
    .mockResolvedValue({ rules, nextKey: '' });
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

  test('does not mention other cloud providers', async () => {
    setupMocks();
    const { container } = renderSettingsTab();
    await waitFor(() => {
      expect(screen.getByText(/Integration Details/i)).toBeInTheDocument();
    });
    expect(container).not.toHaveTextContent(/\bAWS\b/);
  });

  test('shows unsupported notice when rules have different subscriptions', async () => {
    setupMocks([
      { ...baseRule, region: 'eastus', subscriptions: ['sub-a'] },
      { ...baseRule, region: 'westus', subscriptions: ['sub-b'] },
    ]);
    renderSettingsTab();
    await waitFor(() => {
      expect(screen.getByText(/unsupported by this form/i)).toBeInTheDocument();
    });
  });

  test('shows unsupported notice when rules have different resource groups', async () => {
    setupMocks([
      { ...baseRule, region: 'eastus', resourceGroups: ['rg-1'] },
      { ...baseRule, region: 'westus', resourceGroups: ['rg-2'] },
    ]);
    renderSettingsTab();
    await waitFor(() => {
      expect(screen.getByText(/unsupported by this form/i)).toBeInTheDocument();
    });
  });

  test('does not show unsupported notice when rules are from a single matcher', async () => {
    setupMocks([
      { ...baseRule, region: 'eastus' },
      { ...baseRule, region: 'westus' },
    ]);
    renderSettingsTab();
    await waitFor(() => {
      expect(screen.getByText(/Integration Details/i)).toBeInTheDocument();
    });
    expect(
      screen.queryByText(/unsupported by this form/i)
    ).not.toBeInTheDocument();
  });
});
