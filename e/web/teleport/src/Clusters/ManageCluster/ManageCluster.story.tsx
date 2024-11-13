import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport/index';
import { ClusterInfo } from 'teleport/services/clusters';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Route } from 'teleport/components/Router';
import { clusterInfoFixture } from 'teleport/Clusters/fixtures';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';

import { ManageCluster } from './ManageCluster';

export default {
  title: 'TeleportE/Clusters/ManageCluster',
};

function render(
  fetchClusterDetails: (clusterId: string) => Promise<any>,
  getUpgradeWindowStartHour?: (clusterId: string) => Promise<any>
) {
  const ctx = createTeleportContextE();

  ctx.clusterService.fetchClusterDetails = fetchClusterDetails;
  ctx.upgradeWindowService.getUpgradeWindowStartHour =
    getUpgradeWindowStartHour;

  return (
    <MemoryRouter initialEntries={['/clusters/test-cluster']}>
      <Route path="/clusters/:clusterId">
        <ContentMinWidth>
          <ContextProvider ctx={ctx}>
            <ManageCluster />
          </ContextProvider>
        </ContentMinWidth>
      </Route>
    </MemoryRouter>
  );
}

export function Loading() {
  const fetchClusterDetails = () => {
    // promise never resolves to simulate loading state
    return new Promise(() => {});
  };
  return render(fetchClusterDetails);
}

export function LoadingCloud() {
  const getUpgradeWindowStartHour = () => {
    // promise never resolves to simulate loading state
    return new Promise(() => {});
  };
  return render(mockFetchClusterDetailsCloudSuccess, getUpgradeWindowStartHour);
}

export function Failed() {
  const fetchClusterDetails = () =>
    Promise.reject(new Error('Failed to load cluster details'));
  return render(fetchClusterDetails);
}

export function UpgradeWindowFailed() {
  const mockGetUpgradeWindowStartHour = () =>
    Promise.reject(new Error('Failed to load upgrade window start time'));
  return render(
    mockFetchClusterDetailsCloudSuccess,
    mockGetUpgradeWindowStartHour
  );
}

export function SuccessCloud() {
  return render(
    mockFetchClusterDetailsCloudSuccess,
    mockGetUpgradeWindowStartHourSuccess
  );
}

export function SuccessSelfHosted() {
  const mockFetchClusterDetails = () =>
    new Promise(resolve => {
      resolve({ ...clusterInfoFixture, isCloud: false } as ClusterInfo);
    });
  return render(mockFetchClusterDetails);
}

const mockFetchClusterDetailsCloudSuccess = () => {
  return new Promise(resolve => {
    resolve({ ...clusterInfoFixture, isCloud: true });
  });
};

const mockGetUpgradeWindowStartHourSuccess = () => {
  return new Promise(resolve => {
    resolve(8);
  });
};
