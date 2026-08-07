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

import { displayDateTime } from 'design/datetime';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import {
  reportOneOfIsDNSReport,
  reportOneOfIsRouteConflictReport,
  reportOneOfIsSSHConfigurationReport,
} from 'teleterm/helpers';

export const hasReportFoundIssues = (report: diag.Report): boolean =>
  report.checks.some(
    checkAttempt =>
      checkAttempt.status === diag.CheckAttemptStatus.OK &&
      checkAttempt.checkReport.status === diag.CheckReportStatus.ISSUES_FOUND
  );

export const getReportFilename = (report: diag.Report) => {
  const createdAt = displayDateTime(Timestamp.toDate(report.createdAt));
  // Colons are best avoided on macOS and forbidden on Windows.
  // https://en.wikipedia.org/wiki/Filename#Comparison_of_filename_limitations
  // Spaces are removed as well to avoid issues with encoding if the user uploads the file and makes
  // it accessible over a URL that includes the filename.
  const sanitizedCreatedAt = createdAt
    .replaceAll(' ', '_')
    .replaceAll(':', '-');

  return `vnet_diag_report_${sanitizedCreatedAt}.txt`;
};

/**
 * reportToText serializes the report into text that can be shared by the user. It was written
 * primarily with Zendesk and Slack in mind. As of February 2025, Slack doesn't support Markdown
 * tables, but Zendesk does.
 *
 * Still, the text should be light on Markdown as the user might post it to a platform that
 * doesn't support Markdown. For example, we should refrain from using <details>.
 */
export function reportToText(report: diag.Report): string {
  const createdAt = Timestamp.toDate(report.createdAt);
  const localCreatedAt = displayDateTime(createdAt);
  const utcCreatedAt = createdAt.toUTCString();
  const networkStack = networkStackAttemptToText(report.networkStackAttempt);
  const checkReports = report.checks.map(checkAttemptToText).join('\n\n');

  return `VNet Diagnostic Report

Created at: ${localCreatedAt} (${utcCreatedAt})
${networkStack}

${checkReports}\n`;
}

function networkStackAttemptToText(
  networkStackAttempt: diag.NetworkStackAttempt
): string {
  const { networkStack } = networkStackAttempt;

  return networkStackAttempt.status === diag.CheckAttemptStatus.ERROR
    ? `Network details could not be determined:
${networkStackAttempt.error}`
    : `Network interface: ${networkStack.interfaceName}
IPv4 CIDR ranges: ${networkStack.ipv4CidrRanges.join(', ')}
IPv6 prefix: ${networkStack.ipv6Prefix || 'disabled on this host'}
DNS zones: ${networkStack.dnsZones.join(', ')}`;
}

function checkAttemptToText(checkAttempt: diag.CheckAttempt): string {
  const reportOneof = checkAttempt.checkReport.report.oneofKind;
  const displayDetails = reportOneofToDisplayDetails[reportOneof];

  if (!displayDetails) {
    return `Cannot display the result from an unsupported check ${reportOneof}.`;
  }

  let checkSummary: string;
  if (checkAttempt.status === diag.CheckAttemptStatus.ERROR) {
    checkSummary = `Failed to ${displayDetails.errorTitle}:
${checkAttempt.error}`;
  } else {
    checkSummary = displayDetails.reportToText(checkAttempt.checkReport);
  }

  const commandSummaries = checkAttempt.commands.map(commandAttempt =>
    commandAttempt.status === diag.CommandAttemptStatus.ERROR
      ? `Ran into an error when executing ${commandAttempt.command}:
${commandAttempt.error}`
      : `\`\`\`
$ ${commandAttempt.command}
${commandAttempt.output}
\`\`\``
  );

  return `---
${checkSummary}

${commandSummaries.join('\n\n')}`;
}

const reportOneofToDisplayDetails: Record<
  diag.CheckReport['report']['oneofKind'],
  {
    reportToText: (checkReport: diag.CheckReport) => string;
    errorTitle: string;
  }
> = {
  routeConflictReport: {
    errorTitle: 'inspect network routes',
    reportToText: routeConflictReportToText,
  },
  sshConfigurationReport: {
    errorTitle: 'inspect SSH configuration',
    reportToText: sshConfigurationReportToText,
  },
  dnsReport: {
    errorTitle: 'inspect DNS configuration',
    reportToText: dnsReportToText,
  },
};

function routeConflictReportToText({
  report,
  status,
}: diag.CheckReport): string {
  if (!reportOneOfIsRouteConflictReport(report)) {
    return '';
  }

  if (status === diag.CheckReportStatus.OK) {
    return '✅ There are no network routes in conflict with VNet.';
  }

  const tableRows = report.routeConflictReport.routeConflicts
    .map(
      routeConflict =>
        `| ${routeConflict.vnetDest} | ${routeConflict.dest} | ${routeConflict.interfaceName} | ${routeConflict.interfaceApp || 'unknown'} |`
    )
    .join('\n');

  return `⚠️ There are network routes in conflict with VNet.

| VNet destination | Conflicting destination | Interface | Set up by |
| ---------------- | ----------------------- | --------- | --------- |
${tableRows}`;
}

function sshConfigurationReportToText({ report }: diag.CheckReport): string {
  if (!reportOneOfIsSSHConfigurationReport(report)) {
    return null;
  }
  const {
    userOpensshConfigPath,
    vnetSshConfigPath,
    userOpensshConfigIncludesVnetSshConfig,
    userOpensshConfigExists,
    userOpensshConfigContents,
  } = report.sshConfigurationReport;

  const status = userOpensshConfigIncludesVnetSshConfig
    ? '✅ VNet SSH is configured correctly.'
    : `⚠️ VNet SSH is not configured.

  The user's default SSH configuration file does not include VNet's
  generated configuration file and connections to VNet SSH hosts will
  not work by default.`;

  const pathsTable = `
| File description         | Path |
| ------------------------ | ---- |
| User OpenSSH config file | ${userOpensshConfigPath}  |
| VNet SSH config file     | ${vnetSshConfigPath}  |`;

  const currentContents = userOpensshConfigExists
    ? `Current contents of ${userOpensshConfigPath}:

\`\`\`
${userOpensshConfigContents}
\`\`\``
    : `${userOpensshConfigPath} does not exist`;

  return `${status}
${pathsTable}

${currentContents}`;
}

function dnsReportToText({ report }: diag.CheckReport): string {
  if (!reportOneOfIsDNSReport(report)) {
    return '';
  }
  const { ipv4Reachability, ipv6Reachability, zoneResults } = report.dnsReport;

  const reachabilityLines = [
    reachabilityToText('IPv4', ipv4Reachability),
    reachabilityToText('IPv6', ipv6Reachability),
  ].filter(Boolean);

  const allUnreachable =
    (ipv4Reachability || ipv6Reachability) &&
    (!ipv4Reachability || !ipv4Reachability.reachable) &&
    (!ipv6Reachability || !ipv6Reachability.reachable);
  if (allUnreachable) {
    reachabilityLines.push(
      "VNet's DNS is not responding. This might be caused by network routes set up by another program that capture traffic meant for VNet."
    );
  }

  // Show only zones that have at least one problem. A null aRecord
  // or aaaaRecord means that record type wasn't queried because no
  // expected IP was captured from the reachability step, so not a problem.
  const okStatus = diag.DNSZoneStatus.DNS_ZONE_STATUS_OK;
  const problemRows = zoneResults.filter(
    zr =>
      (zr.aRecord && zr.aRecord.status !== okStatus) ||
      (zr.aaaaRecord && zr.aaaaRecord.status !== okStatus)
  );

  if (problemRows.length === 0) {
    return reachabilityLines.join('\n');
  }

  // Drop the record-type column if no row in the table has a result for it.
  const showA = problemRows.some(zr => zr.aRecord);
  const showAaaa = problemRows.some(zr => zr.aaaaRecord);
  const headerCells = ['Zone'];
  if (showA) headerCells.push('A');
  if (showAaaa) headerCells.push('AAAA');
  const headerRow = `| ${headerCells.join(' | ')} |`;
  const separatorRow = `| ${headerCells.map(() => '---').join(' | ')} |`;
  const tableRows = problemRows
    .map(zr => {
      const cells = [zr.zone];
      if (showA) cells.push(recordResultToText(zr.aRecord));
      if (showAaaa) cells.push(recordResultToText(zr.aaaaRecord));
      return `| ${cells.join(' | ')} |`;
    })
    .join('\n');

  const headerCount =
    problemRows.length === 1
      ? `1 of ${zoneResults.length} DNS zones is not routed through VNet.`
      : `${problemRows.length} of ${zoneResults.length} DNS zones are not routed through VNet.`;

  return `${reachabilityLines.join('\n')}

⚠️ ${headerCount}

${headerRow}
${separatorRow}
${tableRows}`;
}

function reachabilityToText(
  family: 'IPv4' | 'IPv6',
  reach: diag.VNetDNSReachability | undefined
): string {
  if (!reach) {
    return '';
  }
  if (!reach.reachable) {
    return `⚠️ VNet ${family} DNS unreachable on ${reach.address}: ${reach.error}`;
  }
  const responded =
    reach.respondedA && reach.respondedAaaa
      ? 'A, AAAA'
      : reach.respondedA
        ? 'A only'
        : reach.respondedAaaa
          ? 'AAAA only'
          : 'nothing';
  return `✅ VNet ${family} DNS reachable on ${reach.address} (responds to ${responded})`;
}

function recordResultToText(rr: diag.RecordResult | undefined): string {
  if (!rr) {
    return '—';
  }
  const label = dnsZoneStatusToText(rr.status);
  if (rr.observedIp) {
    return `${label} (${rr.observedIp})`;
  }
  return label;
}

function dnsZoneStatusToText(status: diag.DNSZoneStatus): string {
  switch (status) {
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_OK:
      return 'OK';
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED:
      return 'hijacked';
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED:
      return 'not registered';
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT:
      return 'timeout';
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_RESOLVER_ERROR:
      return 'resolver error';
    default:
      return `unknown (${status})`;
  }
}
