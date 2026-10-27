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

import { configMocks, mockIntersectionObserver } from 'jsdom-testing-mocks';
import { afterAll, afterEach, beforeAll, expect, test, vi } from 'vitest';

import { act, render, screen } from 'design/utils/testing';

import { ContentMinWidth } from 'teleport/Main/Main';
import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { SamlAppActionProvider } from 'teleport/SamlApplications/useSamlAppActions';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserContext } from 'teleport/User/UserContext';

import { ClusterResources } from './UnifiedResources';

configMocks({ beforeAll, afterAll, afterEach, act });
const intersectionObserver = mockIntersectionObserver();

test('empty state for enterprise or oss, with create permissions', async () => {
  renderResources({ canCreate: true, isLeafCluster: false });

  await expect(
    screen.findByRole('heading', { name: 'Add Your First Resource' })
  ).resolves.toBeVisible();
});

test.each([
  { canCreate: false, isLeafCluster: false },
  { canCreate: true, isLeafCluster: true },
])(
  'read-only empty state when canCreate=$canCreate and isLeafCluster=$isLeafCluster',
  async props => {
    renderResources(props);

    await expect(
      screen.findByText(/Either there are no resources in the cluster/i)
    ).resolves.toBeVisible();
    expect(
      screen.queryByRole('heading', { name: 'Add Your First Resource' })
    ).not.toBeInTheDocument();
  }
);

function renderResources({
  canCreate,
  isLeafCluster,
}: {
  canCreate: boolean;
  isLeafCluster: boolean;
}) {
  const acl = getAcl();
  if (!canCreate) {
    acl.tokens = noAccess;
  }
  const ctx = createTeleportContext({ customAcl: acl });
  ctx.resourceService.fetchUnifiedResources = vi
    .fn()
    .mockResolvedValue({ agents: [] });
  ctx.clusterService.fetchClusters = vi.fn().mockResolvedValue([]);

  render(
    <TeleportProviderBasic teleportCtx={ctx}>
      <ContentMinWidth>
        <UserContext.Provider
          value={{
            preferences: makeDefaultUserPreferences(),
            updatePreferences: vi.fn(),
            getClusterPinnedResources: async () => [],
            updateClusterPinnedResources: vi.fn(),
            updateDiscoverResourcePreferences: vi.fn(),
          }}
        >
          <SamlAppActionProvider>
            <ClusterResources
              clusterId="im-a-cluster"
              isLeafCluster={isLeafCluster}
            />
          </SamlAppActionProvider>
        </UserContext.Provider>
      </ContentMinWidth>
    </TeleportProviderBasic>
  );

  act(intersectionObserver.enterAll);
}
