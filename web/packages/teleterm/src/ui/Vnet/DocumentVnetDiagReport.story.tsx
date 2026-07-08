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

import { Meta } from '@storybook/react-vite';
import { PropsWithChildren, useEffect } from 'react';

import { Box } from 'design';
import {
  ConnectionStat,
  RecentConnectionKind,
} from 'gen-proto-ts/teleport/lib/teleterm/vnet/v1/vnet_service_pb';
import {
  CheckAttemptStatus,
  CheckReportStatus,
  CommandAttemptStatus,
  DNSReport,
  DNSZoneStatus,
  RouteConflict,
  RouteConflictReport,
  SSHConfigurationReport,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { usePromiseRejectedOnUnmount } from 'shared/utils/wait';

import { MockedUnaryCall } from 'teleterm/services/tshd/cloneableClient';
import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';
import { reportToText } from 'teleterm/services/vnet/diag';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeDNSReport,
  makeDNSZoneResult,
  makeRecordResult,
  makeReport,
  makeRouteConflict,
  makeVNetDNSReachability,
} from 'teleterm/services/vnet/testHelpers';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { MockWorkspaceContextProvider } from 'teleterm/ui/fixtures/MockWorkspaceContextProvider';
import { makeDocumentVnetDiagReport } from 'teleterm/ui/services/workspacesService/documentsService/testHelpers';
import { ConnectionsContextProvider } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { DocumentVnetDiagReport as Component } from './DocumentVnetDiagReport';
import { useVnetContext, VnetContextProvider } from './vnetContext';

const defaultUserSSHConfigContents = `Include "/Users/User/Library/Application Support/Teleport Connect/tsh/vnet_ssh_config"

Host github.com
  IdentityFile ~/.ssh/id_ed25519
`;

type StoryProps = {
  asText: boolean;
  networkStackAttempt: 'ok' | 'error';
  ipv4CidrRanges: string[];
  dnsZones: string[];
  routeConflictAttempt: 'ok' | 'issues-found' | 'error';
  routeConflicts: RouteConflict[];
  routeConflictCommandAttempt: 'ok' | 'error';
  dnsAttempt:
    | 'ok'
    | 'unreachable'
    | 'ipv6-unreachable'
    | 'no-ipv6'
    | 'mixed-zone-issues'
    | 'both-aaaa-only'
    | 'error';
  sshConfigAttempt: 'ok' | 'error';
  sshConfigured: boolean;
  userOpenSSHConfigExists: boolean;
  userOpenSSHConfigContents: string;
  displayUnsupportedCheckAttempt: boolean;
  vnetRunning: boolean;
  reRunDiagnostics: 'success' | 'error' | 'processing';
  connectionStats: ConnectionStat[];
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
    dnsAttempt: {
      control: { type: 'inline-radio' },
      options: [
        'ok',
        'unreachable',
        'ipv6-unreachable',
        'no-ipv6',
        'mixed-zone-issues',
        'both-aaaa-only',
        'error',
      ],
    },
    sshConfigAttempt: {
      control: { type: 'inline-radio' },
      options: ['ok', 'error'],
    },
    userOpenSSHConfigExists: {},
    userOpenSSHConfigContents: {},
    sshConfigured: {
      control: { type: 'boolean' },
    },
    displayUnsupportedCheckAttempt: {
      description:
        "Simulate the component receiving a report with a check attempt that's not supported in the current version",
    },
    reRunDiagnostics: {
      control: { type: 'inline-radio' },
      options: ['success', 'error', 'processing'],
    },
    connectionStats: { control: { type: 'object' } },
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
    dnsAttempt: 'mixed-zone-issues',
    sshConfigAttempt: 'ok',
    sshConfigured: false,
    userOpenSSHConfigExists: true,
    userOpenSSHConfigContents: defaultUserSSHConfigContents,
    displayUnsupportedCheckAttempt: false,
    vnetRunning: true,
    reRunDiagnostics: 'success',
    connectionStats: [
      {
        kind: RecentConnectionKind.APP,
        cluster: 'teleport.example.com',
        leafCluster: '',
        displayName: 'grafana.teleport.example.com',
        port: 0,
        successfulConnections: 12n,
        failedConnections: 0n,
        bytesTx: 1_300_000n,
        bytesRx: 15_700_000n,
        bytesTxPerSec: 12_000n,
        bytesRxPerSec: 250_000n,
      },
      {
        kind: RecentConnectionKind.APP,
        cluster: 'teleport.example.com',
        leafCluster: '',
        displayName: 'multiport.teleport.example.com',
        port: 8443,
        successfulConnections: 2n,
        failedConnections: 3n,
        bytesTx: 4_200n,
        bytesRx: 0n,
        bytesTxPerSec: 0n,
        bytesRxPerSec: 0n,
      },
      {
        kind: RecentConnectionKind.SSH,
        cluster: 'teleport.example.com',
        leafCluster: '',
        displayName: 'node-01.teleport.example.com',
        port: 0,
        successfulConnections: 4n,
        failedConnections: 1n,
        bytesTx: 52_000n,
        bytesRx: 1_100_000n,
        bytesTxPerSec: 0n,
        bytesRxPerSec: 0n,
      },
    ],
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

  appContext.vnet.getConnections = (() => ({
    responses: {
      onMessage: (
        callback: (response: { stats: ConnectionStat[] }) => void
      ) => {
        callback({ stats: props.connectionStats });
        return () => {};
      },
      onNext: () => () => {},
      onComplete: () => () => {},
      onError: () => () => {},
    },
    then: () => Promise.resolve(),
  })) as unknown as typeof appContext.vnet.getConnections;

  return (
    <MockAppContextProvider appContext={appContext}>
      <ConnectionsContextProvider>
        <MockWorkspaceContextProvider>
          <VnetContextProvider>{props.children}</VnetContextProvider>
        </MockWorkspaceContextProvider>
      </ConnectionsContextProvider>
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

  const sshConfigReport: SSHConfigurationReport = {
    userOpensshConfigIncludesVnetSshConfig: props.sshConfigured,
    userOpensshConfigPath: '/Users/User/.ssh/config',
    vnetSshConfigPath:
      '/Users/User/Library/Application Support/Teleport Connect/tsh/vnet_ssh_config',
    userOpensshConfigExists: props.userOpenSSHConfigExists,
    userOpensshConfigContents: props.userOpenSSHConfigContents,
  };
  const sshConfigCheckAttempt = makeCheckAttempt({
    status:
      props.sshConfigAttempt === 'ok'
        ? CheckAttemptStatus.OK
        : CheckAttemptStatus.ERROR,
    error:
      props.sshConfigAttempt === 'error' ? 'something went wrong' : undefined,
    checkReport: makeCheckReport({
      status: CheckReportStatus.OK,
      report: {
        oneofKind: 'sshConfigurationReport',
        sshConfigurationReport: sshConfigReport,
      },
    }),
  });
  const dnsReport: DNSReport = makeDNSReport();
  const dnsCheckAttempt = makeCheckAttempt({
    checkReport: makeCheckReport({
      status: CheckReportStatus.OK,
      report: { oneofKind: 'dnsReport', dnsReport },
    }),
  });
  switch (props.dnsAttempt) {
    case 'error':
      dnsCheckAttempt.status = CheckAttemptStatus.ERROR;
      dnsCheckAttempt.error = 'something went wrong';
      break;
    case 'ok':
      // Both nameservers reachable, all zones routed correctly.
      dnsReport.zoneResults = props.dnsZones.map(zone =>
        makeDNSZoneResult({ zone })
      );
      break;
    case 'unreachable':
      // Both nameservers unreachable.
      dnsReport.ipv4Reachability = makeVNetDNSReachability({
        address: '100.64.0.2:53',
        reachable: false,
        respondedA: false,
        respondedAaaa: false,
        error:
          'querying VNet DNS server at 100.64.0.2:53 for A record\n' +
          '\tlookup vnet-diag-f4b49a465de45626.test: connect: connection refused\n' +
          'querying VNet DNS server at 100.64.0.2:53 for AAAA record\n' +
          '\tlookup vnet-diag-aa7634492d2eeb20.test: connect: connection refused',
      });
      dnsReport.ipv6Reachability = makeVNetDNSReachability({
        address: '[fdff:fd74:46c0::2]:53',
        reachable: false,
        respondedA: false,
        respondedAaaa: false,
        error:
          'querying VNet DNS server at [fdff:fd74:46c0::2]:53 for A record\n' +
          '\tlookup vnet-diag-f4b49a465de45626.test: connect: connection refused\n' +
          'querying VNet DNS server at [fdff:fd74:46c0::2]:53 for AAAA record\n' +
          '\tlookup vnet-diag-aa7634492d2eeb20.test: connect: connection refused',
      });
      dnsCheckAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
      break;
    case 'ipv6-unreachable':
      // IPv6 nameserver unreachable, IPv4 fine. Per-zone still runs.
      dnsReport.ipv6Reachability = makeVNetDNSReachability({
        address: '[fdff:fd74:46c0::2]:53',
        reachable: false,
        respondedA: false,
        respondedAaaa: false,
        error:
          'querying VNet DNS server at [fdff:fd74:46c0::2]:53 for A record\n' +
          '\tlookup vnet-diag-e08c7ee8b534d6db.test: connect: network unreachable\n' +
          'querying VNet DNS server at [fdff:fd74:46c0::2]:53 for AAAA record\n' +
          '\tlookup vnet-diag-907e5ae789aea710.test: connect: network unreachable',
      });
      dnsReport.zoneResults = [
        ...props.dnsZones.map(zone => makeDNSZoneResult({ zone })),
        makeDNSZoneResult({
          zone: 'hijack.zone.test',
          aRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '1.2.3.4',
          }),
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '2001:db8::1234',
          }),
        }),
      ];
      dnsCheckAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
      break;
    case 'no-ipv6':
      // VNet does not serve DNS on IPv6.
      dnsReport.ipv6Reachability = undefined;
      dnsReport.ipv4Reachability = makeVNetDNSReachability({
        address: '100.64.0.2:53',
        respondedA: true,
        respondedAaaa: false,
      });
      dnsReport.zoneResults = [
        ...props.dnsZones.map(zone =>
          makeDNSZoneResult({ zone, aaaaRecord: undefined })
        ),
        makeDNSZoneResult({
          zone: 'hijack.zone.test',
          aRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '1.2.3.4',
          }),
          aaaaRecord: undefined,
        }),
      ];
      dnsCheckAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
      break;
    case 'mixed-zone-issues':
      // Per-zone mixed results.
      dnsReport.zoneResults = [
        makeDNSZoneResult({ zone: 'company.test' }),
        makeDNSZoneResult({
          zone: 'hijack.zone.test',
          aRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '1.2.3.4',
          }),
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '2001:db8::1234',
          }),
        }),
        makeDNSZoneResult({
          zone: 'mixed.zone.test',
          aRecord: makeRecordResult(),
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '2001:db8::1234',
          }),
        }),
        makeDNSZoneResult({
          zone: 'not-registered.zone.test',
          aRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED,
          }),
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED,
          }),
        }),
        makeDNSZoneResult({
          zone: 'staging.test',
          aRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT,
            error: 'i/o timeout',
          }),
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT,
            error: 'i/o timeout',
          }),
        }),
      ];
      dnsCheckAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
      break;
    case 'both-aaaa-only':
      // Both nameservers reachable but respond to AAAA only.
      dnsReport.ipv4Reachability = makeVNetDNSReachability({
        address: '100.64.0.2:53',
        respondedA: false,
        respondedAaaa: true,
      });
      dnsReport.ipv6Reachability = makeVNetDNSReachability({
        address: '[fdff:fd74:46c0::2]:53',
        respondedA: false,
        respondedAaaa: true,
      });
      dnsReport.zoneResults = [
        makeDNSZoneResult({ zone: 'company.test', aRecord: undefined }),
        makeDNSZoneResult({
          zone: 'hijack.zone.test',
          aRecord: undefined,
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
            observedIp: '2001:db8::1234',
          }),
        }),
        makeDNSZoneResult({
          zone: 'not-registered.zone.test',
          aRecord: undefined,
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED,
          }),
        }),
        makeDNSZoneResult({
          zone: 'staging.test',
          aRecord: undefined,
          aaaaRecord: makeRecordResult({
            status: DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT,
            error: 'i/o timeout',
          }),
        }),
      ];
      dnsCheckAttempt.checkReport.status = CheckReportStatus.ISSUES_FOUND;
      break;
  }
  dnsCheckAttempt.commands.push({
    status: CommandAttemptStatus.OK,
    error: '',
    command: 'scutil --dns',
    output: scutilOutput,
  });
  report.checks.push(dnsCheckAttempt);
  report.checks.push(sshConfigCheckAttempt);

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

const scutilOutput = `DNS configuration

resolver #1
  nameserver[0] : 1.1.1.1
  nameserver[1] : 192.168.1.1
  if_index : 14 (en0)
  flags    : Request A records
  reach    : 0x00000002 (Reachable)
  order    : 200000

resolver #2
  domain   : local
  options  : mdns
  timeout  : 5
  flags    : Request A records
  reach    : 0x00000000 (Not Reachable)
  order    : 300000

resolver #3
  domain   : company.test
  nameserver[0] : fdff:fd74:46c0::2
  nameserver[1] : 100.64.0.2
  flags    : Request A records
  reach    : 0x00000002 (Reachable)

resolver #4
  domain   : other.test
  nameserver[0] : fdff:fd74:46c0::2
  nameserver[1] : 100.64.0.2
  flags    : Request A records
  reach    : 0x00000002 (Reachable)

DNS configuration (for scoped queries)

resolver #1
  nameserver[0] : 1.1.1.1
  nameserver[1] : 192.168.1.1
  if_index : 14 (en0)
  flags    : Scoped, Request A records
  reach    : 0x00000002 (Reachable)
`;

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
