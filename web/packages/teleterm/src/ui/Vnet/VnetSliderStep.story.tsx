/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import { useEffect } from 'react';

import { Box } from 'design';
import { usePromiseRejectedOnUnmount } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';

import { useVnetContext, VnetContextProvider } from './vnetContext';
import { VnetSliderStep } from './VnetSliderStep';

export default {
  title: 'Teleterm/Vnet/VnetSliderStep',
  decorators: [
    Story => {
      return (
        <Box width={324} bg="levels.elevated">
          <Story />
        </Box>
      );
    },
  ],
};

const dnsZones = ['teleport.example.com', 'company.test'];

export function Running() {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.listDNSZones = () => new MockedUnaryCall({ dnsZones });

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

export function UpdatingDnsZones() {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  const promise = usePromiseRejectedOnUnmount();
  appContext.vnet.listDNSZones = () => promise;

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

export function UpdatingDnsZonesWithPreviousResults() {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  const promise = usePromiseRejectedOnUnmount();
  let firstCall = true;
  appContext.vnet.listDNSZones = () => {
    if (firstCall) {
      firstCall = false;
      return new MockedUnaryCall({ dnsZones });
    }
    return promise;
  };

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <RerequestDNSZones />
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

const RerequestDNSZones = () => {
  const { listDNSZones, listDNSZonesAttempt } = useVnetContext();

  useEffect(() => {
    if (listDNSZonesAttempt.status === 'success') {
      listDNSZones();
    }
  }, [listDNSZonesAttempt, listDNSZones]);

  return null;
};

export function DnsZonesError() {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.listDNSZones = () =>
    new MockedUnaryCall(undefined, new Error('something went wrong'));

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

export function StartError() {
  const appContext = new MockAppContext();
  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.start = () =>
    new MockedUnaryCall(undefined, new Error('something went wrong'));

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

export function UnexpectedShutdown() {
  const appContext = new MockAppContext();

  appContext.statePersistenceService.putState({
    ...appContext.statePersistenceService.getState(),
    vnet: { autoStart: true },
  });
  appContext.workspacesService.setState(draft => {
    draft.isInitialized = true;
  });
  appContext.vnet.start = () => {
    setTimeout(() => {
      appContext.unexpectedVnetShutdownListener({
        error: 'lorem ipsum dolor sit amet',
      });
    }, 0);
    return new MockedUnaryCall({});
  };

  return (
    <MockAppContextProvider appContext={appContext}>
      <VnetContextProvider>
        <Component />
      </VnetContextProvider>
    </MockAppContextProvider>
  );
}

const Component = () => (
  <VnetSliderStep
    refCallback={noop}
    next={noop}
    prev={noop}
    hasTransitionEnded
    stepIndex={1}
    flowLength={2}
  />
);

const noop = () => {};
