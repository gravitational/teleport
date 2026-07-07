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

import { useCallback, useRef } from 'react';
import styled from 'styled-components';

import { Button, Alert as DesignAlert, Flex, H1, Link, Stack } from 'design';
import { AlertProps } from 'design/Alert/Alert';
import Table, { Cell, TextCell } from 'design/DataTable';
import { displayDateTime } from 'design/datetime';
import {
  ChevronRight,
  Copy,
  Download,
  Refresh,
  Check as SuccessIcon,
  Warning as WarningIcon,
} from 'design/Icon';
import { P1, P2 } from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { CanceledError, useAsync } from 'shared/hooks/useAsync';
import { pluralize } from 'shared/utils/text';

import {
  reportOneOfIsDNSReport,
  reportOneOfIsRouteConflictReport,
  reportOneOfIsSSHConfigurationReport,
} from 'teleterm/helpers';
import { getReportFilename, reportToText } from 'teleterm/services/vnet/diag';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import type * as docTypes from 'teleterm/ui/services/workspacesService';

import { useVnetContext } from './vnetContext';

export function DocumentVnetDiagReport(props: {
  visible: boolean;
  doc: docTypes.DocumentVnetDiagReport;
}) {
  const { report } = props.doc;
  const { networkStackAttempt } = report;
  const { networkStack } = networkStackAttempt;
  const createdAt = displayDateTime(Timestamp.toDate(report.createdAt));
  const { notificationsService, mainProcessClient } = useAppContext();
  const { getDisabledDiagnosticsReason, runDiagnostics } = useVnetContext();
  const { documentsService } = useWorkspaceContext();

  // Re-wrap runDiagnostics into another attempt. This has multiple benefits:
  // 1) It captures the result of just this one manual run of diagnostics.
  // 2) It automatically clears any auxiliary state between runs.
  // 3) Running diagnostics from the VNet panel has no effect on any specific tab, but re-running
  //    diagnostics from the tab affects the VNet panel.
  const [manualDiagnosticsAttempt, manualRunDiagnostics] = useAsync(
    useCallback(async () => {
      const [report, error] = await runDiagnostics();
      if (error) {
        // If the manual run is made stale by VNet context executing a periodic run, use the result
        // of the manual run anyway.
        if (error instanceof CanceledError && error.stalePromise) {
          return error.stalePromise as Promise<diag.Report>;
        }
        throw error;
      }
      return report;
    }, [runDiagnostics])
  );
  const runDiagnosticsAndReplaceReport = async () => {
    const [report, error] = await manualRunDiagnostics();
    if (error) {
      return;
    }
    documentsService.update(props.doc.uri, { report });
  };

  const disabledDiagnosticsReason = getDisabledDiagnosticsReason(
    manualDiagnosticsAttempt
  );

  const previousClipboardNotificationIdRef = useRef('');
  const copyReportToClipboard = async () => {
    notificationsService.removeNotification(
      previousClipboardNotificationIdRef.current
    );
    const text = reportToText(report);
    await copyToClipboard(text);
    previousClipboardNotificationIdRef.current =
      notificationsService.notifyInfo('Copied the report to the clipboard.');
  };

  const previousSaveToFileNotificationIdRef = useRef('');
  const saveReportToFile = async () => {
    notificationsService.removeNotification(
      previousSaveToFileNotificationIdRef.current
    );

    const text = reportToText(report);
    let result: Awaited<ReturnType<typeof mainProcessClient.saveTextToFile>>;
    try {
      result = await mainProcessClient.saveTextToFile({
        text,
        defaultBasename: getReportFilename(report),
      });
    } catch (error) {
      previousSaveToFileNotificationIdRef.current =
        notificationsService.notifyError({
          title: 'Could not save the report to a file.',
          description: error?.message,
        });
      return;
    }

    if (!result.canceled) {
      previousSaveToFileNotificationIdRef.current =
        notificationsService.notifyInfo('Saved the report to a file.');
    }
  };

  // Warning headings for every problem in the report, so the collapsed summary tells the user what
  // went wrong without expanding the report.
  const issues = [
    ...getNetworkStackIssues(networkStackAttempt),
    ...report.checks.flatMap(getCheckAttemptIssues),
  ];

  return (
    <Document visible={props.visible}>
      <Stack
        gap={4}
        maxWidth="680px"
        fullWidth
        mx="auto"
        mt={4}
        p={5}
        backgroundColor="levels.surface"
        borderRadius={2}
        // Without this, the Stack would span the whole height of the Document, no matter how much
        // content was displayed in the Stack.
        alignSelf="flex-start"
      >
        <Details>
          <ReportSummary>
            <Stack gap={4} fullWidth alignItems="stretch">
              <Flex
                flexWrap="wrap"
                gap={2}
                justifyContent="space-between"
                alignItems="center"
              >
                <Flex gap={2} alignItems="center">
                  <Chevron />
                  <H1>VNet Diagnostic Report</H1>
                </Flex>

                {/* Clicks on the buttons must not toggle the enclosing <details>. */}
                <Flex gap={2} onClick={e => e.stopPropagation()}>
                  <HoverTooltip
                    tipContent={
                      disabledDiagnosticsReason || 'Run Diagnostics Again'
                    }
                  >
                    <Button
                      intent="neutral"
                      p={1}
                      disabled={!!disabledDiagnosticsReason}
                      onClick={runDiagnosticsAndReplaceReport}
                    >
                      <Refresh size="medium" />
                    </Button>
                  </HoverTooltip>

                  <HoverTooltip tipContent="Copy Report to Clipboard">
                    <Button
                      intent="neutral"
                      p={1}
                      onClick={copyReportToClipboard}
                    >
                      <Copy size="medium" />
                    </Button>
                  </HoverTooltip>

                  <HoverTooltip tipContent="Save Report to File">
                    <Button intent="neutral" p={1} onClick={saveReportToFile}>
                      <Download size="medium" />
                    </Button>
                  </HoverTooltip>
                </Flex>
              </Flex>

              {manualDiagnosticsAttempt.status === 'error' && (
                // Keep re-run failures visible even while collapsed; the wrapper stops the click
                // from toggling the <details>.
                <div onClick={e => e.stopPropagation()}>
                  <Alert
                    kind="danger"
                    details={<P2>{manualDiagnosticsAttempt.statusText}</P2>}
                  >
                    Encountered an error while re-running diagnostics.
                  </Alert>
                </div>
              )}

              <SummaryStatus gap={1} fullWidth alignItems="stretch">
                {issues.length > 0 ? (
                  issues.map(issue => (
                    <P1 m={0} key={issue}>
                      <Warning /> {issue}
                    </P1>
                  ))
                ) : (
                  <P1 m={0}>
                    <Success /> No issues found.
                  </P1>
                )}
              </SummaryStatus>
            </Stack>
          </ReportSummary>

          {/*
            Keep the body in a single wrapper. A closed <details> hides it as one unit; making
            <Details> the flex/gap container would leave the gap after the summary while collapsed.
          */}
          <Stack gap={4} fullWidth alignItems="stretch" mt={4}>
            {networkStackAttempt.status === diag.CheckAttemptStatus.ERROR && (
              <Stack gap={2} fullWidth alignItems="stretch">
                <P2>Created at: {createdAt}</P2>
                <Alert
                  kind="danger"
                  details={<P2>{networkStackAttempt.error}</P2>}
                >
                  {networkDetailsUnavailableTitle}
                </Alert>
              </Stack>
            )}

            {networkStackAttempt.status === diag.CheckAttemptStatus.OK && (
              <P2>
                Created at: {createdAt}
                <br />
                Network interface: <code>{networkStack.interfaceName}</code>
                <br />
                IPv4 CIDR{' '}
                {pluralize(networkStack.ipv4CidrRanges.length, 'range')}:{' '}
                <code>{networkStack.ipv4CidrRanges.join(', ')}</code>
                <br />
                IPv6 prefix: <code>{networkStack.ipv6Prefix}</code>
                <br />
                DNS {pluralize(networkStack.dnsZones.length, 'zone')}:{' '}
                <code>{networkStack.dnsZones.join(', ')}</code>
              </P2>
            )}

            {report.checks.map(checkAttempt => (
              <CheckAttempt
                // tshd promises that checkAttempt.checkReport.report.oneofKind is
                // 1) always present even if the check fails to complete
                // 2) unique
                key={checkAttempt.checkReport.report.oneofKind}
                checkAttempt={checkAttempt}
              />
            ))}
          </Stack>
        </Details>
      </Stack>
    </Document>
  );
}

/**
 * CheckAttempt displays the result of attempting to run an individual check along with the outputs
 * of the accompanying commands. The commands are displayed even if the check itself failed to run.
 */
const CheckAttempt = ({
  checkAttempt,
}: {
  checkAttempt: diag.CheckAttempt;
}) => {
  const reportOneof = checkAttempt.checkReport.report.oneofKind;
  const displayDetails = reportOneofDisplayDetails[reportOneof];

  return (
    <Stack gap={2} fullWidth>
      {!displayDetails ? (
        <Alert kind="danger">{unsupportedCheckTitle(reportOneof)}</Alert>
      ) : (
        <>
          {checkAttempt.status === diag.CheckAttemptStatus.ERROR && (
            <Alert kind="danger" details={<P2>{checkAttempt.error}</P2>}>
              {failedToTitle(displayDetails.errorTitle)}
            </Alert>
          )}

          {checkAttempt.status === diag.CheckAttemptStatus.OK && (
            <displayDetails.Component checkReport={checkAttempt.checkReport} />
          )}
        </>
      )}

      {checkAttempt.commands.map(commandAttempt =>
        commandAttempt.status === diag.CommandAttemptStatus.ERROR ? (
          <Alert
            kind="danger"
            key={commandAttempt.command}
            details={<P2>{commandAttempt.error}</P2>}
          >
            Ran into an error when executing{' '}
            <code>{commandAttempt.command}</code>
          </Alert>
        ) : (
          <details key={commandAttempt.command}>
            <Summary>
              <code>{commandAttempt.command}</code>
            </Summary>

            <Pre>{commandAttempt.output}</Pre>
          </details>
        )
      )}
    </Stack>
  );
};

const reportOneofDisplayDetails: Record<
  diag.CheckReport['report']['oneofKind'],
  {
    Component: React.ComponentType<{ checkReport: diag.CheckReport }>;
    errorTitle: string;
    // getIssues returns the warning heading(s) for the problems the check found, matching what
    // Component renders, so the collapsed summary never drifts from the expanded body.
    getIssues: (checkReport: diag.CheckReport) => string[];
  }
> = {
  routeConflictReport: {
    errorTitle: 'inspect network routes',
    Component: CheckReportRouteConflict,
    getIssues: getRouteConflictIssues,
  },
  sshConfigurationReport: {
    errorTitle: 'inspect SSH configuration',
    Component: CheckReportSSHConfiguration,
    getIssues: getSSHConfigurationIssues,
  },
  dnsReport: {
    errorTitle: 'inspect DNS configuration',
    Component: CheckReportDNS,
    getIssues: getDNSIssues,
  },
};

const networkDetailsUnavailableTitle =
  'Network details could not be determined';

const failedToTitle = (errorTitle: string) => `Failed to ${errorTitle}`;

const unsupportedCheckTitle = (reportOneof: string) =>
  `Cannot display the result from an unsupported check ${reportOneof}`;

function getNetworkStackIssues(
  networkStackAttempt: diag.NetworkStackAttempt
): string[] {
  return networkStackAttempt.status === diag.CheckAttemptStatus.ERROR
    ? [networkDetailsUnavailableTitle]
    : [];
}

function getCheckAttemptIssues(checkAttempt: diag.CheckAttempt): string[] {
  const reportOneof = checkAttempt.checkReport.report.oneofKind;
  const displayDetails = reportOneofDisplayDetails[reportOneof];
  if (!displayDetails) {
    return [unsupportedCheckTitle(reportOneof)];
  }
  if (checkAttempt.status === diag.CheckAttemptStatus.ERROR) {
    return [failedToTitle(displayDetails.errorTitle)];
  }
  return displayDetails.getIssues(checkAttempt.checkReport);
}

const routeConflictWarning = (conflictCount: number) =>
  conflictCount === 1
    ? 'There is a network route in conflict with VNet.'
    : 'There are multiple network routes in conflict with VNet.';

function getRouteConflictIssues({
  report,
  status,
}: diag.CheckReport): string[] {
  if (
    !reportOneOfIsRouteConflictReport(report) ||
    status === diag.CheckReportStatus.OK
  ) {
    return [];
  }
  return [
    routeConflictWarning(report.routeConflictReport.routeConflicts.length),
  ];
}

/**
 * CheckReportRouteConflict displays a table with network routes in the system that are in conflict
 * with routes set up by VNet.
 */
function CheckReportRouteConflict({
  checkReport: { report, status },
}: {
  checkReport: diag.CheckReport;
}) {
  if (!reportOneOfIsRouteConflictReport(report)) {
    return null;
  }

  if (status === diag.CheckReportStatus.OK) {
    return (
      <P1>
        <Success /> There are no network routes in conflict with VNet.
      </P1>
    );
  }

  const { routeConflicts } = report.routeConflictReport;

  return (
    <>
      <Stack>
        <P1>
          <Warning /> {routeConflictWarning(routeConflicts.length)}
        </P1>

        <P2 m={0}>
          This might cause the traffic meant for VNet to be captured by another
          interface. The cluster admin might be able to resolve this problem by{' '}
          <Link
            target="_blank"
            href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/#configuring-ipv4-cidr-range"
          >
            adjusting the IPv4 CIDR range used by VNet
          </Link>
          .
        </P2>
      </Stack>

      <Table
        emptyText=""
        data={routeConflicts}
        columns={[
          { key: 'vnetDest', headerText: 'VNet destination' },
          { key: 'dest', headerText: 'Conflicting destination' },
          { key: 'interfaceName', headerText: 'Interface' },
          {
            key: 'interfaceApp',
            headerText: 'Set up by',
            render: routeConflict => (
              <TextCell data={routeConflict.interfaceApp || 'unknown'} />
            ),
          },
        ]}
        row={{ getStyle: () => ({ fontFamily: 'monospace' }) }}
      />
    </>
  );
}

const sshNotConfiguredWarning = 'VNet SSH is not configured.';

function getSSHConfigurationIssues({ report }: diag.CheckReport): string[] {
  if (!reportOneOfIsSSHConfigurationReport(report)) {
    return [];
  }
  return report.sshConfigurationReport.userOpensshConfigIncludesVnetSshConfig
    ? []
    : [sshNotConfiguredWarning];
}

function CheckReportSSHConfiguration({
  checkReport: { report },
}: {
  checkReport: diag.CheckReport;
}) {
  const { openSSHConfigurationModal } = useVnetContext();
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
  const pathsTable = (
    <Table
      emptyText=""
      data={[
        {
          desc: 'User OpenSSH config file',
          path: userOpensshConfigPath,
        },
        {
          desc: 'VNet SSH config file',
          path: vnetSshConfigPath,
        },
      ]}
      columns={[
        { key: 'desc', headerText: 'File description' },
        {
          key: 'path',
          headerText: 'Path',
          render: row => (
            <Cell>
              <code>{row.path}</code>
            </Cell>
          ),
        },
      ]}
    />
  );
  if (userOpensshConfigIncludesVnetSshConfig) {
    return (
      <>
        <Stack>
          <P1>
            <Success /> VNet SSH is configured correctly.
          </P1>
          <P2>
            The user's default SSH configuration file correctly includes VNet's
            generated configuration file.
          </P2>
        </Stack>
        {pathsTable}
      </>
    );
  }
  return (
    <>
      <Stack>
        <P1>
          <Warning /> {sshNotConfiguredWarning}
        </P1>
        <P2 m={0}>
          The user's default SSH configuration file does not include VNet's
          generated SSH configuration file. SSH clients will not be able to make
          connections to VNet SSH addresses by default.{' '}
        </P2>
        <Button
          intent="neutral"
          onClick={() =>
            openSSHConfigurationModal({
              vnetSSHConfigPath: vnetSshConfigPath,
            })
          }
        >
          Resolve
        </Button>
      </Stack>
      {userOpensshConfigExists ? (
        <details>
          <Summary>
            Current contents of <code>{userOpensshConfigPath}</code>
          </Summary>
          <Pre>{userOpensshConfigContents}</Pre>
        </details>
      ) : null}
    </>
  );
}

const dnsUnreachableWarning = (family: 'IPv4' | 'IPv6', address: string) =>
  `VNet ${family} DNS unreachable on ${address}.`;

const dnsZonesNotRoutedWarning = (problemCount: number, totalZones: number) =>
  problemCount === 1
    ? `1 of ${totalZones} DNS zones is not fully routed through VNet.`
    : `${problemCount} of ${totalZones} DNS zones are not fully routed through VNet.`;

function getDNSIssues({ report }: diag.CheckReport): string[] {
  if (!reportOneOfIsDNSReport(report)) {
    return [];
  }
  const { ipv4Reachability, ipv6Reachability, zoneResults } = report.dnsReport;
  const issues: string[] = [];
  if (ipv4Reachability && !ipv4Reachability.reachable) {
    issues.push(dnsUnreachableWarning('IPv4', ipv4Reachability.address));
  }
  if (ipv6Reachability && !ipv6Reachability.reachable) {
    issues.push(dnsUnreachableWarning('IPv6', ipv6Reachability.address));
  }
  const problemRows = dnsProblemRows(zoneResults);
  if (problemRows.length > 0) {
    issues.push(
      dnsZonesNotRoutedWarning(problemRows.length, zoneResults.length)
    );
  }
  return issues;
}

// Show only zones that have at least one problem. A null aRecord or aaaaRecord means that record
// type wasn't queried because no expected IP was captured from the reachability step, so not a
// problem.
function dnsProblemRows(
  zoneResults: diag.DNSZoneResult[]
): diag.DNSZoneResult[] {
  const okStatus = diag.DNSZoneStatus.DNS_ZONE_STATUS_OK;
  return zoneResults.filter(
    zr =>
      (zr.aRecord && zr.aRecord.status !== okStatus) ||
      (zr.aaaaRecord && zr.aaaaRecord.status !== okStatus)
  );
}

/**
 * CheckReportDNS displays per-family reachability and the per-zone, per-record-type outcomes.
 */
function CheckReportDNS({
  checkReport: { report },
}: {
  checkReport: diag.CheckReport;
}) {
  if (!reportOneOfIsDNSReport(report)) {
    return null;
  }
  const { ipv4Reachability, ipv6Reachability, zoneResults } = report.dnsReport;

  const problemRows = dnsProblemRows(zoneResults);

  const allUnreachable =
    (ipv4Reachability || ipv6Reachability) &&
    (!ipv4Reachability || !ipv4Reachability.reachable) &&
    (!ipv6Reachability || !ipv6Reachability.reachable);

  return (
    <Stack>
      <ReachabilityRow family="IPv4" reach={ipv4Reachability} />
      <ReachabilityRow family="IPv6" reach={ipv6Reachability} />

      {allUnreachable && (
        <P2 m={0}>
          VNet's DNS is not responding. This might be caused by network routes
          set up by another program that capture traffic meant for VNet.
        </P2>
      )}

      {problemRows.length > 0 && (
        <>
          <Stack>
            <P1>
              <Warning />{' '}
              {dnsZonesNotRoutedWarning(problemRows.length, zoneResults.length)}
            </P1>
            <P2 m={0}>
              The OS resolver is not sending some queries for the zones below to
              VNet.
            </P2>
          </Stack>
          <Table
            emptyText=""
            data={problemRows}
            // Drop the record-type column if no row in the table has a result for it.
            columns={[
              { key: 'zone' as const, headerText: 'Zone' },
              ...(problemRows.some(zr => zr.aRecord)
                ? [
                    {
                      key: 'aRecord' as const,
                      headerText: 'A',
                      render: (zr: diag.DNSZoneResult) => (
                        <RecordResultCell rr={zr.aRecord} />
                      ),
                    },
                  ]
                : []),
              ...(problemRows.some(zr => zr.aaaaRecord)
                ? [
                    {
                      key: 'aaaaRecord' as const,
                      headerText: 'AAAA',
                      render: (zr: diag.DNSZoneResult) => (
                        <RecordResultCell rr={zr.aaaaRecord} />
                      ),
                    },
                  ]
                : []),
            ]}
            row={{ getStyle: () => ({ fontFamily: 'monospace' }) }}
          />
        </>
      )}
    </Stack>
  );
}

function ReachabilityRow({
  family,
  reach,
}: {
  family: 'IPv4' | 'IPv6';
  reach: diag.VNetDNSReachability | undefined;
}) {
  // Unset reachability means VNet doesn't serve DNS on this family. Skip the row.
  if (!reach) {
    return null;
  }
  if (!reach.reachable) {
    return (
      <Stack gap={1}>
        <P1 m={0}>
          <Warning /> VNet {family} DNS unreachable on{' '}
          <code>{reach.address}</code>. The error:
        </P1>
        <ErrorPre>{reach.error}</ErrorPre>
      </Stack>
    );
  }
  const responded =
    reach.respondedA && reach.respondedAaaa
      ? 'A, AAAA'
      : reach.respondedA
        ? 'A only'
        : reach.respondedAaaa
          ? 'AAAA only'
          : 'nothing';
  return (
    <P1 m={0}>
      <Success /> VNet {family} DNS reachable on <code>{reach.address}</code>{' '}
      (responds to {responded})
    </P1>
  );
}

function RecordResultCell({ rr }: { rr: diag.RecordResult | undefined }) {
  // No expected IP was captured for this record type, the cell is left empty.
  if (!rr) {
    return <TextCell data="" />;
  }
  return (
    <Cell>
      <DnsZoneStatusLabel status={rr.status} />
      {rr.observedIp ? ` (${rr.observedIp})` : null}
    </Cell>
  );
}

function DnsZoneStatusLabel({ status }: { status: diag.DNSZoneStatus }) {
  switch (status) {
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_OK:
      return <Success />;
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_HIJACKED:
      return <>Hijacked</>;
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_NOT_REGISTERED:
      return <>Not registered</>;
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_TIMEOUT:
      return <>Timeout</>;
    case diag.DNSZoneStatus.DNS_ZONE_STATUS_RESOLVER_ERROR:
      return <>Resolver error</>;
    default:
      return <>Unknown ({status})</>;
  }
}

const Details = styled.details`
  width: 100%;
  margin: 0;
`;

const ReportSummary = styled.summary`
  cursor: pointer;
  list-style: none;

  &::-webkit-details-marker {
    display: none;
  }
`;

// The at-a-glance issue lines are only shown while collapsed; once the report is expanded, the
// check results below carry the same information.
const SummaryStatus = styled(Stack)`
  details[open] & {
    display: none;
  }
`;

const Chevron = styled(ChevronRight).attrs({ size: 'medium' })`
  flex-shrink: 0;
  transition: transform 150ms ease;

  details[open] & {
    transform: rotate(90deg);
  }
`;

const Summary = styled.summary`
  cursor: pointer;
`;

const Pre = styled.pre`
  white-space: pre-wrap;
`;

const ErrorPre = styled(Pre)`
  margin: 0;
  align-self: stretch;
  line-height: 1.6;
  tab-size: 4;
  padding: ${props => props.theme.space[2]}px;
  background-color: ${props => props.theme.colors.levels.sunken};
  border-radius: ${props => props.theme.radii[2]}px;
  font-size: ${props => props.theme.fontSizes[1]}px;
`;

const Warning = styled(WarningIcon).attrs({
  size: 'small',
  color: 'interactive.solid.alert.default',
})`
  vertical-align: sub;
`;

const Success = styled(SuccessIcon).attrs({
  size: 'small',
  color: 'interactive.solid.success.default',
})`
  vertical-align: sub;
`;

const Alert = (props: Pick<AlertProps, 'children' | 'details' | 'kind'>) => (
  <DesignAlert m={0} {...props} />
);
