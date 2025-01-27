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
import { Meta } from '@storybook/react';
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

import { useVnetContext, VnetContextProvider } from './vnetContext';
import { VnetSliderStep as Component } from './VnetSliderStep';

type StoryProps = {
  startVnet: 'success' | 'error' | 'processing';
  autoStart: boolean;
  dnsZones: string[];
  listDnsZones:
    | 'success'
    | 'error'
    | 'processing'
    | 'processing-with-previous-results';
  vnetDiag: boolean;
  runDiagnostics: 'success' | 'error' | 'processing';
  diagReport: 'ok' | 'issues-found' | 'failed-checks';
  isWorkspacePresent: boolean;
  unexpectedShutdown: boolean;
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
    dnsZones: {
      control: { type: 'object' },
    },
    listDnsZones: {
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
  args: {
    startVnet: 'success',
    autoStart: true,
    dnsZones: ['teleport.example.com', 'company.test'],
    listDnsZones: 'success',
    vnetDiag: true,
    runDiagnostics: 'success',
    diagReport: 'ok',
    isWorkspacePresent: true,
    unexpectedShutdown: false,
  },
};
export default meta;

export function VnetSliderStep(props: StoryProps) {
  const appContext = new MockAppContext();

  if (props.isWorkspacePresent) {
    appContext.addRootCluster(makeRootCluster());
  }

  if (props.autoStart) {
    appContext.statePersistenceService.putState({
      ...appContext.statePersistenceService.getState(),
      vnet: { autoStart: true },
    });
    appContext.workspacesService.setState(draft => {
      draft.isInitialized = true;
    });
  }

  if (props.vnetDiag) {
    appContext.configService.set('unstable.vnetDiag', true);
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

  if (props.listDnsZones === 'processing') {
    appContext.vnet.listDNSZones = () => pendingPromise;
  } else {
    let firstCall = true;
    appContext.vnet.listDNSZones = () => {
      if (props.listDnsZones === 'processing-with-previous-results') {
        if (firstCall) {
          firstCall = false;
          return new MockedUnaryCall({ dnsZones: props.dnsZones });
        }
        return pendingPromise;
      }

      return new MockedUnaryCall(
        { dnsZones: props.dnsZones },
        props.listDnsZones === 'error'
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
      <VnetContextProvider>
        {props.listDnsZones === 'processing-with-previous-results' && (
          <RerequestDNSZones />
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

const noop = () => {};
