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

import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { reportToText } from './diag';
import {
  makeCheckAttempt,
  makeCheckReport,
  makeCommandAttempt,
  makeDNSReport,
  makeDNSZoneResult,
  makeRecordResult,
  makeReport,
  makeRouteConflict,
  makeVNetDNSReachability,
} from './testHelpers';

describe('reportToText', () => {
  it('converts report correctly', () => {
    const routeConflictReport = makeCheckReport({
      status: diag.CheckReportStatus.ISSUES_FOUND,
    });
    routeConflictReport.report = {
      oneofKind: 'routeConflictReport',
      routeConflictReport: {
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
      },
    };
    const dnsCheckReport = makeCheckReport({
      status: diag.CheckReportStatus.ISSUES_FOUND,
      report: {
        oneofKind: 'dnsReport',
        dnsReport: makeDNSReport({
          ipv6Reachability: makeVNetDNSReachability({
            address: '[fdff:fd74:46c0::2]:53',
            reachable: false,
            respondedA: false,
            respondedAaaa: false,
            error:
              'dial udp [fdff:fd74:46c0::2]:53: connect: network unreachable',
          }),
          zoneResults: [
            makeDNSZoneResult({ zone: 'company.test' }),
            makeDNSZoneResult({
              zone: 'hijack.zone.test',
              aRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
                observedIp: '1.2.3.4',
              }),
              aaaaRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
                observedIp: '2001:db8::1234',
              }),
            }),
            makeDNSZoneResult({
              zone: 'mixed.zone.test',
              aRecord: makeRecordResult(),
              aaaaRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED,
                observedIp: '2001:db8::1234',
              }),
            }),
            makeDNSZoneResult({
              zone: 'not-registered.zone.test',
              aRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED,
              }),
              aaaaRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED,
              }),
            }),
            makeDNSZoneResult({
              zone: 'staging.test',
              aRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT,
                error: 'i/o timeout',
              }),
              aaaaRecord: makeRecordResult({
                status: diag.DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT,
                error: 'i/o timeout',
              }),
            }),
          ],
        }),
      },
    });
    const sshConfigReport = makeCheckReport({
      status: diag.CheckReportStatus.OK,
    });
    sshConfigReport.report = {
      oneofKind: 'sshConfigurationReport',
      sshConfigurationReport: {
        userOpensshConfigPath: '~/.ssh/config',
        vnetSshConfigPath:
          '/Users/user/Library/Application Support/Teleport Connect/tsh/vnet_ssh_config',
        userOpensshConfigIncludesVnetSshConfig: false,
        userOpensshConfigExists: false,
        userOpensshConfigContents: '',
      },
    };
    const report = makeReport({
      checks: [
        makeCheckAttempt({
          checkReport: routeConflictReport,
          commands: [makeCommandAttempt()],
        }),
        makeCheckAttempt({
          checkReport: dnsCheckReport,
        }),
        makeCheckAttempt({
          checkReport: sshConfigReport,
        }),
      ],
    });

    const actualText = reportToText(report);
    expect(actualText).toMatchSnapshot();
    // Verify that the text ends with a newline.
    expect(actualText.endsWith('\n')).toBe(true);
  });

  it('renders DNS report when both nameservers are unreachable', () => {
    const dnsCheckReport = makeCheckReport({
      status: diag.CheckReportStatus.ISSUES_FOUND,
      report: {
        oneofKind: 'dnsReport',
        dnsReport: makeDNSReport({
          ipv4Reachability: makeVNetDNSReachability({
            address: '100.64.0.2:53',
            reachable: false,
            respondedA: false,
            respondedAaaa: false,
            error: 'dial udp 100.64.0.2:53: connect: connection refused',
          }),
          ipv6Reachability: makeVNetDNSReachability({
            address: '[fdff:fd74:46c0::2]:53',
            reachable: false,
            respondedA: false,
            respondedAaaa: false,
            error:
              'dial udp [fdff:fd74:46c0::2]:53: connect: connection refused',
          }),
          zoneResults: [],
        }),
      },
    });
    const report = makeReport({
      checks: [makeCheckAttempt({ checkReport: dnsCheckReport })],
    });

    const actualText = reportToText(report);
    expect(actualText).toMatchSnapshot();
    expect(actualText.endsWith('\n')).toBe(true);
  });
});
