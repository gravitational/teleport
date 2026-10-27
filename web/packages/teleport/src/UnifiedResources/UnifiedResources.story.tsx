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

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useMemo, useState } from 'react';
import styled from 'styled-components';

import { ContentMinWidth } from 'teleport/Main/Main';
import {
  createTeleportContext,
  getAcl,
  noAccess,
} from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { makeDefaultUserPreferences } from 'teleport/services/userPreferences/userPreferences';
import { UserContext } from 'teleport/User/UserContext';

import { UnifiedResources as Component } from './UnifiedResources';

const meta = {
  title: 'Teleport/UnifiedResources',
  component: Component,
} satisfies Meta<typeof Component>;

type Story = StoryObj<typeof meta>;

export default meta;

export const EmptyState: Story = {
  render: () => <UnifiedResourcesStory canCreate={true} />,
};

export const ReadOnlyEmptyState: Story = {
  render: () => <UnifiedResourcesStory canCreate={false} />,
};

function UnifiedResourcesStory({ canCreate }: { canCreate: boolean }) {
  const ctx = useMemo(() => {
    const acl = getAcl();
    if (!canCreate) {
      acl.tokens = noAccess;
    }

    const ctx = createTeleportContext({ customAcl: acl });
    ctx.resourceService.fetchUnifiedResources = async () => ({ agents: [] });
    return ctx;
  }, [canCreate]);
  const [preferences, setPreferences] = useState(makeDefaultUserPreferences);

  return (
    <TeleportProviderBasic teleportCtx={ctx}>
      <FullHeight>
        <ContentMinWidth>
          <UserContext.Provider
            value={{
              preferences,
              updatePreferences: async updates => {
                setPreferences(current => ({ ...current, ...updates }));
              },
              getClusterPinnedResources: async () => [],
              updateClusterPinnedResources: async () => undefined,
              updateDiscoverResourcePreferences: async () => undefined,
            }}
          >
            <Component />
          </UserContext.Provider>
        </ContentMinWidth>
      </FullHeight>
    </TeleportProviderBasic>
  );
}

const FullHeight = styled.div`
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  display: flex;
  flex-direction: column;
`;
