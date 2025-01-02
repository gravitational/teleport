import { MemoryRouter, Route } from 'react-router-dom';

import { render, screen, waitFor } from 'design/utils/testing';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { clusterInfoFixture } from 'teleport/Clusters/fixtures';
import { ContextProvider } from 'teleport/index';
import { ContentMinWidth } from 'teleport/Main/Main';

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

  test('fetches upgrade window for Cloud clusters', async () => {
    const ctx = createTeleportContextE();
    jest
      .spyOn(ctx.clusterService, 'fetchClusterDetails')
      .mockResolvedValueOnce({ ...clusterInfoFixture, isCloud: true });

    jest
      .spyOn(ctx.upgradeWindowService, 'getUpgradeWindowStartHour')
      .mockResolvedValueOnce(8);

    renderElement(<ManageCluster />, ctx);
    await waitFor(() => {
      expect(screen.getByText(/Cluster Information/)).toBeInTheDocument();
    });

    expect(
      screen.getByText(clusterInfoFixture.authVersion)
    ).toBeInTheDocument();
    expect(screen.getByText(clusterInfoFixture.clusterId)).toBeInTheDocument();
    expect(screen.getByText(clusterInfoFixture.publicURL)).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText(/Scheduled Upgrades/)).toBeInTheDocument();
    });

    expect(screen.getByText('08:00 (UTC)')).toBeInTheDocument();

    expect(ctx.clusterService.fetchClusterDetails).toHaveBeenCalledTimes(1);
    expect(
      ctx.upgradeWindowService.getUpgradeWindowStartHour
    ).toHaveBeenCalledTimes(1);
  });

  test('does not fetch upgrade window for non-Cloud clusters', async () => {
    const ctx = createTeleportContextE();
    jest
      .spyOn(ctx.clusterService, 'fetchClusterDetails')
      .mockResolvedValueOnce({ ...clusterInfoFixture, isCloud: false });

    jest
      .spyOn(ctx.upgradeWindowService, 'getUpgradeWindowStartHour')
      .mockResolvedValueOnce(8);

    renderElement(<ManageCluster />, ctx);

    await waitFor(() => {
      expect(screen.getByText(/Cluster Information/)).toBeInTheDocument();
    });

    expect(
      screen.getByText(clusterInfoFixture.authVersion)
    ).toBeInTheDocument();
    expect(screen.getByText(clusterInfoFixture.clusterId)).toBeInTheDocument();

    // upgrade window should not be on the screen
    expect(screen.queryByText(/Scheduled Upgrades/)).not.toBeInTheDocument();

    expect(ctx.clusterService.fetchClusterDetails).toHaveBeenCalledTimes(1);
    // upgrade window service should not have been called
    expect(
      ctx.upgradeWindowService.getUpgradeWindowStartHour
    ).not.toHaveBeenCalled();
  });
});
