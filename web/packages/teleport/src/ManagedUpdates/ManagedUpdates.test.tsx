/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { http, HttpResponse } from 'msw';
import { PropsWithChildren } from 'react';

import {
  enableMswServer,
  Providers,
  render,
  screen,
  server,
  waitFor,
} from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'teleport/config';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { TeleportProviderBasic } from 'teleport/mocks/providers';
import { defaultAccess, makeAcl } from 'teleport/services/user/makeAcl';

import {
  mockManagedUpdatesAgentsDropped,
  mockManagedUpdatesHaltOnError,
  mockManagedUpdatesImmediate,
  mockManagedUpdatesTimeBased,
  mockManagedUpdatesWithOrphaned,
} from './fixtures';
import { ManagedUpdates } from './ManagedUpdates';

enableMswServer();

function makeWrapper(customAcl?: ReturnType<typeof makeAcl>) {
  return ({ children }: PropsWithChildren) => {
    const ctx = createTeleportContext({ customAcl });
    return (
      <Providers>
        <TeleportProviderBasic teleportCtx={ctx}>
          <InfoGuidePanelProvider>{children}</InfoGuidePanelProvider>
        </TeleportProviderBasic>
      </Providers>
    );
  };
}

describe('ManagedUpdates', () => {
  test('time-based strategy doesnt show order column in table', async () => {
    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesTimeBased)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Rollout Group')).toBeInTheDocument();
    });

    expect(screen.queryByText('Order')).not.toBeInTheDocument();
  });

  test('halt-on-error strategy should show order column in table', async () => {
    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesHaltOnError)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper() });

    await waitFor(() => {
      expect(screen.getByText('Order')).toBeInTheDocument();
    });
  });

  test('immediate schedule shows info alert instead of table', async () => {
    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesImmediate)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper() });

    await waitFor(() => {
      expect(
        screen.getByText(
          /every group immediately updates to the target version/i
        )
      ).toBeInTheDocument();
    });

    expect(screen.queryByText('Rollout Group')).not.toBeInTheDocument();
  });

  test('dropped agents detected shows warning', async () => {
    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesAgentsDropped)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper() });

    await waitFor(() => {
      expect(
        screen.getByText('Action required. Hover for details.')
      ).toBeInTheDocument();
    });
  });

  test('ungrouped agents shows count in the last groups row', async () => {
    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesWithOrphaned)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper() });

    await waitFor(() => {
      expect(screen.getByText('+ 25 ungrouped instances')).toBeInTheDocument();
    });
  });

  test('missing permissions shows alert banner', async () => {
    const noPermissionsAcl = makeAcl({
      autoUpdateConfig: defaultAccess,
      autoUpdateVersion: defaultAccess,
      autoUpdateAgentRollout: defaultAccess,
    });

    server.use(
      http.get(cfg.getManagedUpdatesUrl(), () =>
        HttpResponse.json(mockManagedUpdatesTimeBased)
      )
    );

    render(<ManagedUpdates />, { wrapper: makeWrapper(noPermissionsAcl) });

    await waitFor(() => {
      expect(
        screen.getByText(/do not have all the required permissions/i)
      ).toBeInTheDocument();
    });
  });
});
