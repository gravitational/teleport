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

import { within } from '@testing-library/react';
import { afterEach, expect, test, vi } from 'vitest';

import { render, screen, userEvent, waitFor } from 'design/utils/testing';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';
import type { Resource } from 'teleport/services/resources';

import TrustedClusters from './TrustedClusters';

afterEach(() => {
  vi.restoreAllMocks();
});

test('renders trusted clusters before a resource is selected', async () => {
  renderTrustedClusters();

  expect(await screen.findByText(cluster.name)).toBeInTheDocument();
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
});

test('closing the editor clears the selection and allows reopening it', async () => {
  const user = userEvent.setup();
  renderTrustedClusters();

  await user.click(
    await screen.findByRole('button', { name: 'Edit Trusted Cluster' })
  );

  expect(screen.getByText('Edit trusted root cluster')).toBeInTheDocument();
  expect(
    within(screen.getByRole('dialog')).getByText(cluster.name)
  ).toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Cancel' }));

  expect(
    screen.queryByText('Edit trusted root cluster')
  ).not.toBeInTheDocument();
  expect(screen.getByText(cluster.name)).toBeInTheDocument();

  await user.click(
    screen.getByRole('button', { name: 'Edit Trusted Cluster' })
  );

  expect(
    within(screen.getByRole('dialog')).getByText(cluster.name)
  ).toBeInTheDocument();
});

test('removing a selected cluster returns to the empty state', async () => {
  const user = userEvent.setup();
  const { ctx } = renderTrustedClusters();
  const remove = vi
    .spyOn(ctx.resourceService, 'deleteTrustedCluster')
    .mockResolvedValue(undefined);

  await screen.findByText(cluster.name);
  await user.click(screen.getByTestId('button'));
  await user.click(screen.getByRole('menuitem', { name: 'Delete...' }));

  expect(screen.getByText('Remove Trusted Cluster?')).toBeInTheDocument();

  await user.click(
    screen.getByRole('button', { name: 'Yes, Remove Trusted Cluster' })
  );

  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  );
  expect(remove).toHaveBeenCalledWith(cluster.name);
  expect(screen.queryByText(cluster.name)).not.toBeInTheDocument();
  expect(
    screen.getByRole('button', { name: 'Connect to Root Cluster' })
  ).toBeEnabled();
});

function renderTrustedClusters() {
  const ctx = createTeleportContext();
  vi.spyOn(ctx.resourceService, 'fetchTrustedClusters').mockResolvedValue([
    cluster,
  ]);

  render(
    <ContextProvider ctx={ctx}>
      <TrustedClusters />
    </ContextProvider>
  );

  return { ctx };
}

const cluster: Resource<'trusted_cluster'> = {
  id: 'trusted_cluster:dummy-trusted-cluster',
  kind: 'trusted_cluster',
  name: 'dummy-trusted-cluster',
  content: 'kind: trusted_cluster',
};
