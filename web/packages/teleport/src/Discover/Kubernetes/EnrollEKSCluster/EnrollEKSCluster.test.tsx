/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { act, fireEvent, render, screen } from 'design/utils/testing';

import cfg from 'teleport/config';
import { ComponentWrapper } from 'teleport/Discover/Fixtures/kubernetes';
import auth from 'teleport/services/auth';
import * as discoveryService from 'teleport/services/discovery/discovery';
import {
  DEFAULT_DISCOVERY_GROUP_NON_CLOUD,
  DISCOVERY_GROUP_CLOUD,
} from 'teleport/services/discovery/discovery';
import {
  AwsEksCluster,
  integrationService,
} from 'teleport/services/integrations';
import KubeService from 'teleport/services/kube/kube';
import { userEventService } from 'teleport/services/userEvent';
import { ProxyRequiresUpgrade } from 'teleport/services/version/unsupported';

import { EnrollEksCluster } from './EnrollEksCluster';

const defaultIsCloud = cfg.isCloud;

describe('test EnrollEksCluster.tsx', () => {
  let createDiscoveryConfig;
  beforeEach(() => {
    cfg.isCloud = true;
    jest
      .spyOn(KubeService.prototype, 'fetchKubernetes')
      .mockResolvedValue({ agents: [] });
    jest
      .spyOn(userEventService, 'captureDiscoverEvent')
      .mockResolvedValue(undefined as never);
    jest
      .spyOn(auth, 'getMfaChallengeResponseForAdminAction')
      .mockResolvedValue(undefined);
    createDiscoveryConfig = jest
      .spyOn(discoveryService, 'createDiscoveryConfig')
      .mockResolvedValue({
        name: '',
        discoveryGroup: '',
        aws: [],
      });
  });

  afterEach(() => {
    cfg.isCloud = defaultIsCloud;
    jest.resetAllMocks();
  });

  test('without EKS clusters available, does not attempt to fetch kube clusters', async () => {
    jest
      .spyOn(integrationService, 'fetchEksClusters')
      .mockResolvedValue({ clusters: [] });

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // No results are rendered.
    await screen.findByText(/no result/i);

    expect(integrationService.fetchEksClusters).toHaveBeenCalledTimes(1);
    expect(KubeService.prototype.fetchKubernetes).not.toHaveBeenCalled();
  });

  test('with EKS clusters available, makes a fetch request for kube clusters', async () => {
    jest.spyOn(integrationService, 'fetchEksClusters').mockResolvedValue({
      clusters: mockEKSClusters,
    });

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    // EKS results are rendered.
    await screen.findByText(/eks1/i);

    expect(integrationService.fetchEksClusters).toHaveBeenCalledTimes(1);
    expect(KubeService.prototype.fetchKubernetes).toHaveBeenCalledTimes(1);
  });

  test('auto enroll (cloud) is on by default', async () => {
    jest.spyOn(integrationService, 'fetchEksClusters').mockResolvedValue({
      clusters: mockEKSClusters,
    });
    jest.spyOn(integrationService, 'enrollEksClustersV2');

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByText(/eks1/i);

    // Toggle on auto enroll.
    act(() => screen.getByText(/auto-enroll all/i).click());

    // Cloud uses a default discovery group name.
    expect(
      screen.queryByText(/define a discovery group name/i)
    ).not.toBeInTheDocument();

    act(() => screen.getByText('Next').click());
    await screen.findByText(/Creating Auto Discovery Config/i);
    expect(discoveryService.createDiscoveryConfig).toHaveBeenCalledTimes(1);

    // 2D array:
    // First array is the array of calls, we are only interested in the first.
    // Second array are the parameters that this api got called with,
    // we are interested in the second parameter.
    expect(createDiscoveryConfig.mock.calls[0][1]['discoveryGroup']).toEqual(
      DISCOVERY_GROUP_CLOUD
    );

    expect(integrationService.enrollEksClustersV2).not.toHaveBeenCalled();
  });

  test('auto enroll (self-hosted) is on by default', async () => {
    cfg.isCloud = false;
    jest.spyOn(integrationService, 'fetchEksClusters').mockResolvedValue({
      clusters: mockEKSClusters,
    });
    jest.spyOn(integrationService, 'enrollEksClustersV2');

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByText(/eks1/i);

    // Toggle on auto enroll.
    act(() => screen.getByText(/auto-enroll all/i).click());
    expect(screen.queryByText(/eks1/i)).not.toBeInTheDocument();

    // Only self-hosted need to define a discovery group name.
    await screen.findByText(/define a discovery group name/i);
    // There should be no table rendered.
    expect(screen.queryByText(/eks1/i)).not.toBeInTheDocument();

    act(() => screen.getByText('Next').click());
    await screen.findByText(/Creating Auto Discovery Config/i);
    expect(discoveryService.createDiscoveryConfig).toHaveBeenCalledTimes(1);

    // 2D array:
    // First array is the array of calls, we are only interested in the first.
    // Second array are the parameters that this api got called with,
    // we are interested in the second parameter.
    expect(createDiscoveryConfig.mock.calls[0][1]['discoveryGroup']).toBe(
      DEFAULT_DISCOVERY_GROUP_NON_CLOUD
    );

    expect(integrationService.enrollEksClustersV2).not.toHaveBeenCalled();
  });

  test('auto enroll disabled, enrolls cluster without labels', async () => {
    jest.spyOn(integrationService, 'fetchEksClusters').mockResolvedValue({
      clusters: mockEKSClusters,
    });
    jest
      .spyOn(integrationService, 'enrollEksClustersV2')
      .mockResolvedValue({} as any); // value doesn't matter
    jest
      .spyOn(integrationService, 'enrollEksClusters')
      .mockResolvedValue({} as any); // value doesn't matter

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByText(/eks1/i);

    act(() => screen.getByRole('radio').click());

    act(() => screen.getByText('Enroll EKS Cluster').click());

    await screen.findByTestId('dialogbox');

    expect(discoveryService.createDiscoveryConfig).not.toHaveBeenCalled();
    expect(KubeService.prototype.fetchKubernetes).toHaveBeenCalledTimes(1);
    expect(integrationService.enrollEksClustersV2).toHaveBeenCalledTimes(1);
    expect(integrationService.enrollEksClusters).not.toHaveBeenCalled();
  });

  test('enroll eks without labels with v1 fallback', async () => {
    jest.spyOn(integrationService, 'fetchEksClusters').mockResolvedValue({
      clusters: mockEKSClusters,
    });
    jest
      .spyOn(integrationService, 'enrollEksClustersV2')
      .mockRejectedValueOnce(new Error(ProxyRequiresUpgrade));
    jest.spyOn(integrationService, 'enrollEksClusters');

    render(<Component />);

    // select a region from selector.
    const selectEl = screen.getByLabelText(/aws region/i);
    fireEvent.focus(selectEl);
    fireEvent.keyDown(selectEl, { key: 'ArrowDown', keyCode: 40 });
    fireEvent.click(screen.getByText('us-east-2'));

    await screen.findByText(/eks1/i);

    act(() => screen.getByRole('radio').click());
    act(() => screen.getByText('Enroll EKS Cluster').click());

    expect(integrationService.enrollEksClustersV2).toHaveBeenCalledTimes(1);

    await screen.findByTestId('dialogbox');

    expect(integrationService.enrollEksClustersV2).toHaveBeenCalledTimes(1);
    expect(integrationService.enrollEksClusters).toHaveBeenCalledTimes(1);
  });
});

const mockEKSClusters: AwsEksCluster[] = [
  {
    name: 'EKS1',
    region: 'us-east-2',
    accountId: '1234567890',
    status: 'active',
    labels: [],
    joinLabels: [],
    authenticationMode: 'API',
    endpointPublicAccess: true,
  },
];

const Component = () => (
  <ComponentWrapper>
    <EnrollEksCluster />
  </ComponentWrapper>
);
