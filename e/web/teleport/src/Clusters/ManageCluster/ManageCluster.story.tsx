import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport/index';
import { ClusterInfo } from 'teleport/services/clusters';
import { ContentMinWidth } from 'teleport/Main/Main';
import { Route } from 'teleport/components/Router';
import { clusterInfoFixture } from 'teleport/Clusters/fixtures';

import { createTeleportContextE } from 'e-teleport/mocks/contexts';
import { Contact } from 'e-teleport/services/contacts/types';
import { upgradeWindowService } from 'e-teleport/services/upgradeWindow';
import { contactsService } from 'e-teleport/services/contacts';

import { ManageCluster } from './ManageCluster';
import { contacts } from './Contacts/fixtures';

export default {
  title: 'TeleportE/Clusters/ManageCluster',
};

function render(
  fetchClusterDetails: (clusterId: string) => Promise<ClusterInfo>,
  getUpgradeWindowStartHour?: (clusterId: string) => Promise<8 | 16 | 23>,
  fetchContacts?: (clusterId: string) => Promise<Contact[]>
) {
  let ctx = createTeleportContextE();
  ctx.storeUser.getContactsAccess = () => ({
    list: true,
    create: true,
    remove: true,
    edit: true,
    read: true,
  });

  ctx.clusterService.fetchClusterDetails = fetchClusterDetails;
  ctx.upgradeWindowService = {
    ...upgradeWindowService,
    getUpgradeWindowStartHour,
  };
  ctx.contactService = { ...contactsService, fetchContacts };

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
  return render(mockLoading);
}

export function LoadingCloud() {
  return render(
    mockFetchClusterDetailsCloudSuccess,
    mockLoading,
    mockFetchContactsSuccess
  );
}

export function LoadingContacts() {
  return render(
    mockFetchClusterDetailsCloudSuccess,
    mockGetUpgradeWindowStartHourSuccess,
    mockLoading
  );
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
    mockGetUpgradeWindowStartHour,
    mockFetchContactsSuccess
  );
}

export function SuccessCloud() {
  return render(
    mockFetchClusterDetailsCloudSuccess,
    mockGetUpgradeWindowStartHourSuccess,
    mockFetchContactsSuccess
  );
}

export function SuccessSelfHosted() {
  const mockFetchClusterDetails = (): Promise<ClusterInfo> =>
    new Promise(resolve => {
      resolve({ ...clusterInfoFixture, isCloud: false } as ClusterInfo);
    });
  return render(mockFetchClusterDetails, null, mockFetchContactsSuccess);
}

const mockFetchClusterDetailsCloudSuccess = (): Promise<ClusterInfo> => {
  return new Promise(resolve => {
    resolve({ ...clusterInfoFixture, isCloud: true });
  });
};

const mockGetUpgradeWindowStartHourSuccess = (): Promise<8> => {
  return new Promise(resolve => {
    resolve(8);
  });
};

const mockFetchContactsSuccess = (): Promise<Contact[]> => {
  return new Promise(resolve => {
    resolve(contacts);
  });
};

const mockLoading = (): Promise<any> => {
  // promise never resolves to simulate loading state
  return new Promise(() => {});
};
