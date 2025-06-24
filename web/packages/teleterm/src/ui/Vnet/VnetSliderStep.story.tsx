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
import { Meta, StoryObj } from '@storybook/react';
import { useEffect } from 'react';

import { Box } from 'design';
import {
  CheckAttemptStatus,
  CheckReportStatus,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { usePromiseRejectedOnUnmount } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { makeReport } from 'teleterm/services/vnet/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { useVnetContext, VnetContextProvider } from './vnetContext';
import { VnetSliderStep as Component } from './VnetSliderStep';

type StoryProps = {
  startVnet: 'success' | 'error' | 'processing';
  autoStart: boolean;
  appDnsZones: string[];
  clusters: string[];
  sshConfigured: boolean;
  fetchStatus:
    | 'success'
    | 'error'
    | 'processing'
    | 'processing-with-previous-results';
  runDiagnostics: 'success' | 'error' | 'processing';
  diagReport: 'ok' | 'issues-found' | 'failed-checks';
  isWorkspacePresent: boolean;
  unexpectedShutdown: boolean;
};

const defaultArgs: StoryProps = {
  startVnet: 'success',
  autoStart: true,
  appDnsZones: ['teleport.example.com', 'company.test'],
  clusters: ['teleport.example.com'],
  sshConfigured: false,
  fetchStatus: 'success',
  runDiagnostics: 'success',
  diagReport: 'ok',
  isWorkspacePresent: true,
  unexpectedShutdown: false,
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/Vnet/VnetSliderStep',
  component: VnetSliderStep,
  decorators: [
    Story => {
      return (
        <Box width={396} bg="levels.elevated">
          <Story />
        </Box>
      );
    },
  ],
  argTypes: {
    startVnet: {
      control: { type: 'inline-radio' },
      options: ['success', 'error', 'processing'],
    },
    appDnsZones: {
      control: { type: 'object' },
    },
    clusters: {
      control: { type: 'object' },
    },
    fetchStatus: {
      control: { type: 'inline-radio' },
      options: [
        'success',
        'error',
        'processing',
        'processing-with-previous-results',
      ],
    },
    runDiagnostics: {
      control: { type: 'inline-radio' },
      options: ['success', 'error', 'processing'],
    },
    diagReport: {
      control: { type: 'inline-radio' },
      options: ['ok', 'issues-found', 'failed-checks'],
    },
    isWorkspacePresent: {
      description:
        "If there's no workspace, the button to open the diag report is disabled.",
    },
  },
  render: props => <VnetSliderStep {...props} />,
};
export default meta;

function VnetSliderStep(props: StoryProps) {
  const appContext = new MockAppContext();

  if (props.isWorkspacePresent) {
    appContext.addRootCluster(makeRootCluster());
  }

  if (props.autoStart) {
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true, hasEverStarted: true },
    });
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
  }

  const pendingPromise = usePromiseRejectedOnUnmount();

  if (props.startVnet === 'processing') {
    appContext.vnet.start = () => pendingPromise;
  } else {
    appContext.vnet.start = () => {
      if (props.startVnet === 'success' && props.unexpectedShutdown) {
        setTimeout(() => {
          appContext.unexpectedVnetShutdownListener({
            error: 'lorem ipsum dolor sit amet',
          });
        }, 5);
      }
      return new MockedUnaryCall(
        {},
        props.startVnet === 'error'
          ? new Error('something went wrong')
          : undefined
      );
    };
  }

  if (props.fetchStatus === 'processing') {
    appContext.vnet.getServiceInfo = () => pendingPromise;
  } else {
    let firstCall = true;
    appContext.vnet.getServiceInfo = () => {
      if (props.fetchStatus === 'processing-with-previous-results') {
        if (firstCall) {
          firstCall = false;
          return new MockedUnaryCall({
            appDnsZones: props.appDnsZones,
            clusters: props.clusters,
            sshConfigured: props.sshConfigured,
          });
        }
        return pendingPromise;
      }

      return new MockedUnaryCall(
        {
          appDnsZones: props.appDnsZones,
          clusters: props.clusters,
          sshConfigured: props.sshConfigured,
        },
        props.fetchStatus === 'error'
          ? new Error('something went wrong')
          : undefined
      );
    };
  }

  if (props.runDiagnostics === 'processing') {
    appContext.vnet.runDiagnostics = () => pendingPromise;
  } else {
    const report = makeReport();
    const checkAttempt = report.checks[0];

    if (props.diagReport === 'issues-found') {
      checkAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
    }

    if (props.diagReport === 'failed-checks') {
      checkAttempt.status = CheckAttemptStatus.ERROR;
      checkAttempt.error = 'something went wrong';
      checkAttempt.checkReport = undefined;
    }

    appContext.vnet.runDiagnostics = () =>
      new MockedUnaryCall(
        { report },
        props.runDiagnostics === 'error'
          ? new Error('something went wrong')
          : undefined
      );
  }

  return (
    <MockAppContextProvider
      appContext={appContext}
      // Completely re-mount everything when controls change. This ensures that effects to fetch
      // data are fired again.
      key={JSON.stringify(props)}
    >
      <ConnectionsContextProvider>
        <VnetContextProvider>
          {props.fetchStatus === 'processing-with-previous-results' && (
            <RerequestServiceInfo />
          )}
          <Component
            refCallback={noop}
            next={noop}
            prev={noop}
            hasTransitionEnded
            stepIndex={1}
            flowLength={2}
          />
        </VnetContextProvider>
      </ConnectionsContextProvider>
    </MockAppContextProvider>
  );
}

const RerequestServiceInfo = () => {
  const { getServiceInfo, serviceInfoAttempt } = useVnetContext();

  useEffect(() => {
    if (serviceInfoAttempt.status === 'success') {
      getServiceInfo();
    }
  }, [serviceInfoAttempt, getServiceInfo]);

  return null;
};

const noop = () => {};

export const CloudCustomer: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: ['example.teleport.sh'],
    clusters: ['example.teleport.sh'],
  },
};

export const SelfHostedWithDifferentClusterName: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: ['teleport.example.com'],
    clusters: ['teleport-example'],
  },
};

export const SelfHostedWithEqualNameAndLeaf: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: ['teleport.example.com', 'leaf.example.com'],
    clusters: ['teleport.example.com', 'leaf.example.com'],
  },
};

export const SelfHostedWithEqualNameAndDifferentLeaf: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: ['teleport.example.com', 'leaf.example.com'],
    clusters: ['teleport.example.com', 'teleport-leaf'],
  },
};

export const SelfHostedWithEqualNameAndCustomDNSZones: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: ['teleport.example.com', 'company.com', 'apps.company'],
    clusters: ['teleport.example.com'],
  },
};

export const SelfHostedWithManyLeavesAndZones: StoryObj<StoryProps> = {
  args: {
    ...defaultArgs,
    appDnsZones: [
      'teleport.example.com',
      'leaf.example.com',
      'second-leaf.example.com',
      'company.com',
    ],
    clusters: [
      'teleport.example.com',
      'teleport-leaf',
      'second-leaf.example.com',
    ],
  },
};
