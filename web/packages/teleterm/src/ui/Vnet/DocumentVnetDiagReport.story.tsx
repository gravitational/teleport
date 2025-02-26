/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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
import { PropsWithChildren, useEffect } from 'react';

import { Box } from 'design';
import {
  CheckAttemptStatus,
  CheckReportStatus,
  CommandAttemptStatus,
  RouteConflict,
  RouteConflictReport,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { usePromiseRejectedOnUnmount } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { reportToText } from 'teleterm/services/vnet/diag';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeReport,
  makeRouteConflict,
} from 'teleterm/services/vnet/testHelpers';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentVnetDiagReport } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';

import { DocumentVnetDiagReport as Component } from './DocumentVnetDiagReport';
import { useVnetContext, VnetContextProvider } from './vnetContext';

type StoryProps = {
  asText: boolean;
  networkStackAttempt: 'ok' | 'error';
  ipv4CidrRanges: string[];
  dnsZones: string[];
  routeConflictAttempt: 'ok' | 'issues-found' | 'error';
  routeConflicts: RouteConflict[];
  routeConflictCommandAttempt: 'ok' | 'error';
  displayUnsupportedCheckAttempt: boolean;
  vnetRunning: boolean;
  reRunDiagnostics: 'success' | 'error' | 'processing';
};

const meta: Meta<StoryProps> = {
  title: 'Teleterm/Vnet/DocumentVnetDiagReport',
  component: DocumentVnetDiagReport,
  decorators: (Story, { args }) => {
    return (
      <Decorator {...args}>
        <Story />
      </Decorator>
    );
  },
  argTypes: {
    asText: {
      description:
        'Render the report as text rather than as a React component.',
    },
    networkStackAttempt: {
      control: { type: 'inline-radio' },
      options: ['ok', 'error'],
    },
    ipv4CidrRanges: { control: { type: 'object' } },
    dnsZones: { control: { type: 'object' } },
    routeConflictAttempt: {
      control: { type: 'inline-radio' },
      options: ['ok', 'issues-found', 'error'],
    },
    routeConflicts: { control: { type: 'object' } },
    routeConflictCommandAttempt: {
      control: { type: 'inline-radio' },
      options: ['ok', 'error'],
    },
    displayUnsupportedCheckAttempt: {
      description:
        "Simulate the component receiving a report with a check attempt that's not supported in the current version",
    },
    reRunDiagnostics: {
      control: { type: 'inline-radio' },
      options: ['success', 'error', 'processing'],
    },
  },
  args: {
    asText: false,
    networkStackAttempt: 'ok',
    ipv4CidrRanges: ['100.64.0.0/10'],
    dnsZones: ['teleport.example.com', 'company.test'],
    routeConflictAttempt: 'issues-found',
    routeConflicts: [
      makeRouteConflict({
        dest: '100.64.0.0/10',
        vnetDest: '100.64.0.1',
        interfaceName: 'utun5',
        interfaceApp: 'VPN: Foobar',
      }),
      makeRouteConflict({
        dest: '0.0.0.0/1',
        vnetDest: '100.64.0.0/10',
        interfaceName: 'utun6',
        interfaceApp: '',
      }),
    ],
    routeConflictCommandAttempt: 'ok',
    displayUnsupportedCheckAttempt: false,
    vnetRunning: true,
    reRunDiagnostics: 'success',
  },
};
export default meta;

const Decorator = (props: PropsWithChildren<StoryProps>) => {
  const appContext = new MockAppContext();
  appContext.addRootCluster(makeRootCluster());

  const pendingPromise = usePromiseRejectedOnUnmount();

  if (props.reRunDiagnostics === 'processing') {
    appContext.vnet.runDiagnostics = () => pendingPromise;
  } else {
    appContext.vnet.runDiagnostics = () =>
      new MockedUnaryCall(
        { report: makeReport() },
        props.reRunDiagnostics === 'error'
          ? new Error('something went wrong')
          : undefined
      );
  }

  return (
    <MockAppContextProvider appContext={appContext}>
      <MockWorkspaceContextProvider>
        <VnetContextProvider>{props.children}</VnetContextProvider>
      </MockWorkspaceContextProvider>
    </MockAppContextProvider>
  );
};

export function DocumentVnetDiagReport(props: StoryProps) {
  const report = makeReport({ checks: [] });
  if (props.networkStackAttempt === 'error') {
    report.networkStackAttempt.status = CheckAttemptStatus.ERROR;
    report.networkStackAttempt.error = 'something went wrong';
    report.networkStackAttempt.networkStack = undefined;
  }
  if (props.networkStackAttempt === 'ok') {
    report.networkStackAttempt.networkStack.ipv4CidrRanges =
      props.ipv4CidrRanges;
    report.networkStackAttempt.networkStack.dnsZones = props.dnsZones;
  }

  const routeConflictReport: RouteConflictReport = { routeConflicts: [] };
  const routeConflictCheckAttempt = makeCheckAttempt({
    checkReport: makeCheckReport({
      report: {
        oneofKind: 'routeConflictReport',
        routeConflictReport,
      },
    }),
  });
  if (props.routeConflictAttempt === 'error') {
    routeConflictCheckAttempt.status = CheckAttemptStatus.ERROR;
    routeConflictCheckAttempt.error = 'something went wrong';
  } else {
    routeConflictCheckAttempt.status = CheckAttemptStatus.OK;
    routeConflictCheckAttempt.checkReport.status = CheckReportStatus.OK;
    if (props.routeConflictAttempt === 'issues-found') {
      routeConflictCheckAttempt.checkReport.status =
        CheckReportStatus.ISSUES_FOUND;
      routeConflictReport.routeConflicts = props.routeConflicts;
    }
  }
  if (props.routeConflictCommandAttempt === 'error') {
    routeConflictCheckAttempt.commands.push({
      status: CommandAttemptStatus.ERROR,
      error: 'something went wrong',
      command: 'netstat -rn -f inet',
      output: '',
    });
  } else {
    routeConflictCheckAttempt.commands.push({
      status: CommandAttemptStatus.OK,
      error: '',
      command: 'netstat -rn -f inet',
      output: netstatOutput,
    });
  }
  report.checks.push(routeConflictCheckAttempt);

  if (props.displayUnsupportedCheckAttempt) {
    report.checks.push({
      status: CheckAttemptStatus.OK,
      checkReport: {
        report: { oneofKind: 'bazBarFooReport' as any },
        status: CheckReportStatus.ISSUES_FOUND,
      },
      error: '',
      commands: [],
    });
  }

  const doc = makeDocumentVnetDiagReport({
    report,
  });
  const { documentsService } = useWorkspaceContext();
  const { start, stop } = useVnetContext();

  // This effect is just so that re-running diagnostics doesn't crash due to missing doc.
  // Re-running diagnostics does not replace the document rendered by the story.
  useEffect(() => {
    documentsService.add(doc);

    return () => {
      documentsService.close(doc.uri);
    };
  }, [documentsService, doc]);

  useEffect(() => {
    if (!props.vnetRunning) {
      return;
    }

    start();

    return () => {
      stop();
    };
  }, [props.vnetRunning, start, stop]);

  if (props.asText) {
    return (
      <Box backgroundColor="levels.surface" p={2}>
        <pre>{reportToText(report)}</pre>
      </Box>
    );
  }

  return <Component visible doc={doc} />;
}

const netstatOutput = `Routing tables

Internet:
Destination        Gateway            Flags               Netif Expire
default            192.168.1.1        UGdScg                en0       
default            link#23            UCSIg           bridge100      !
default            link#25            UCSIg               utun4       
100.64/10          link#25            UCS                 utun4       
100.64.0.1         100.64.0.1         UH                  utun5       
100.87.112.117     100.87.112.117     UH                  utun4       
100.100.100.100/32 link#25            UCS                 utun4       
100.100.100.100    link#25            UHWIi               utun4       
127                127.0.0.1          UCS                   lo0       
127.0.0.1          127.0.0.1          UH                    lo0       
169.254            link#14            UCS                   en0      !
169.254.11.121     50:ec:50:ed:89:cd  UHLSW                 en0      !
169.254.169.254    link#14            UHRLSW                en0      !
172.20.10.2        link#23            UHRLWIg         bridge100     16
172.20.10.2        link#25            UHW3Ig              utun4      6
192.168.1          link#14            UCS                   en0      !
192.168.1.1/32     link#14            UCS                   en0      !
192.168.1.1        7c:10:c9:b5:da:f8  UHLWIir               en0   1188
192.168.1.29       link#14            UHLWI                 en0      !
192.168.1.37       0:5:cd:b0:91:ce    UHLWI                 en0   1181
192.168.1.54       0:11:32:64:d7:d3   UHLWIi                en0   1185
192.168.1.121      2a:d0:62:b8:f6:e2  UHLWI                 en0   1027
192.168.1.183/32   link#14            UCS                   en0      !
192.168.1.183      8e:62:82:7f:23:cb  UHLWI                 lo0       
192.168.1.247      6a:8:72:ed:38:34   UHLWI                 en0   1072
192.168.1.255      ff:ff:ff:ff:ff:ff  UHLWbI                en0      !
192.168.64         link#23            UC              bridge100      !
192.168.64.1       fa.4d.89.68.e8.64  UHLWI                 lo0       
192.168.64.10      12.15.6e.c9.b5.8f  UHLWI           bridge100      !
192.168.64.255     ff.ff.ff.ff.ff.ff  UHLWbI          bridge100      !
224.0.0/4          link#14            UmCS                  en0      !
224.0.0/4          link#25            UmCSI               utun4       
224.0.0.251        1:0:5e:0:0:fb      UHmLWI                en0       
224.0.0.251        1:0:5e:0:0:fb      UHmLWIg         bridge100       
239.255.255.250    1:0:5e:7f:ff:fa    UHmLWI                en0       
255.255.255.255/32 link#14            UCS                   en0      !
255.255.255.255/32 link#25            UCSI                utun4       
`;
