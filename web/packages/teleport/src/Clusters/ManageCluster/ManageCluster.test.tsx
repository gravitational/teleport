import React from 'react';
import { MemoryRouter, Route } from 'react-router-dom';

import { render, waitFor, screen } from 'design/utils/testing';

import { ContextProvider } from 'teleport/index';
import { createTeleportContext } from 'teleport/mocks/contexts';
import { ContentMinWidth } from 'teleport/Main/Main';

import { clusterInfoFixture } from '../fixtures';

import { ManageCluster } from './ManageCluster';

function renderElement(element, ctx) {
  return render(
    <MemoryRouter initialEntries={[`/clusters/cluster-id`]}>
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>{element}</ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

describe('test ManageCluster component', () => {
  beforeEach(() => {
    jest.resetAllMocks();
    jest.spyOn(console, 'error').mockImplementation();
  });

  test('fetches cluster information on load', async () => {
    const ctx = createTeleportContext();
    jest
      .spyOn(ctx.clusterService, 'fetchClusterDetails')
      .mockResolvedValueOnce({ ...clusterInfoFixture });

    renderElement(<ManageCluster />, ctx);
    await waitFor(() => {
      expect(
        screen.getByText(clusterInfoFixture.authVersion)
      ).toBeInTheDocument();
    });

    expect(screen.getByText(clusterInfoFixture.clusterId)).toBeInTheDocument();
    expect(screen.getByText(clusterInfoFixture.publicURL)).toBeInTheDocument();

    expect(ctx.clusterService.fetchClusterDetails).toHaveBeenCalledTimes(1);
  });

  test('shows error when load fails', async () => {
    const ctx = createTeleportContext();
    jest
      .spyOn(ctx.clusterService, 'fetchClusterDetails')
      .mockRejectedValue({ message: 'error message' });

    renderElement(<ManageCluster />, ctx);
    await waitFor(() => {
      expect(
        screen.queryByText(clusterInfoFixture.authVersion)
      ).not.toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('error message')).toBeInTheDocument();
    });

    expect(ctx.clusterService.fetchClusterDetails).toHaveBeenCalledTimes(1);
  });
});
