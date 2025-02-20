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

import styled from 'styled-components';

import { Button, Alert as DesignAlert, Flex, H1, Link, Stack } from 'design';
import { AlertProps } from 'design/Alert/Alert';
import Table, { TextCell } from 'design/DataTable';
import { displayDateTime } from 'design/datetime';
import {
  Copy,
  Download,
  Refresh,
  Check as SuccessIcon,
  Warning as WarningIcon,
} from 'design/Icon';
import { P1, P2 } from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { pluralize } from 'shared/utils/text';

import { reportOneOfIsRouteConflictReport } from 'teleterm/helpers';
import Document from 'teleterm/ui/Document';
import type * as docTypes from 'teleterm/ui/services/workspacesService';

export function DocumentVnetDiagReport(props: {
  visible: boolean;
  doc: docTypes.DocumentVnetDiagReport;
}) {
  const { report } = props.doc;
  const { networkStackAttempt } = report;
  const { networkStack } = networkStackAttempt;
  const createdAt = displayDateTime(Timestamp.toDate(report.createdAt));

  return (
    <Document visible={props.visible}>
      <Stack
        gap={4}
        maxWidth="680px"
        width="100%"
        mx="auto"
        mt={4}
        p={5}
        backgroundColor="levels.surface"
        borderRadius={2}
        // Without this, the Stack would span the whole height of the Document, no matter how much
        // content was displayed in the Stack.
        alignSelf="flex-start"
      >
        <Stack gap={2} width="100%" alignItems="stretch">
          <Flex
            flexWrap="wrap"
            width="100%"
            gap={2}
            justifyContent="space-between"
          >
            <H1>VNet Diagnostic Report</H1>

            <Flex gap={2}>
              {/* TODO(ravicious): Implement buttons. */}
              <HoverTooltip tipContent="Run Diagnostics Again">
                <Button intent="neutral" p={1}>
                  <Refresh size="medium" />
                </Button>
              </HoverTooltip>

              <HoverTooltip tipContent="Copy Report to Clipboard">
                <Button intent="neutral" p={1}>
                  <Copy size="medium" />
                </Button>
              </HoverTooltip>

              <HoverTooltip tipContent="Save Report to File">
                <Button intent="neutral" p={1}>
                  <Download size="medium" />
                </Button>
              </HoverTooltip>
            </Flex>
          </Flex>

          {networkStackAttempt.status === diag.CheckAttemptStatus.ERROR && (
            <>
              <P2>Created at: {createdAt}</P2>
              <Alert
                kind="danger"
                details={<P2>{networkStackAttempt.error}</P2>}
              >
                Network details could not be determined.
              </Alert>
            </>
          )}

          {networkStackAttempt.status === diag.CheckAttemptStatus.OK && (
            <P2>
              Created at: {createdAt}
              <br />
              Network interface: <code>{networkStack.interfaceName}</code>
              <br />
              IPv4 CIDR {pluralize(networkStack.ipv4CidrRanges.length, 'range')}
              : <code>{networkStack.ipv4CidrRanges.join(', ')}</code>
              <br />
              IPv6 prefix: <code>{networkStack.ipv6Prefix}</code>
              <br />
              DNS {pluralize(networkStack.dnsZones.length, 'zone')}:{' '}
              <code>{networkStack.dnsZones.join(', ')}</code>
            </P2>
          )}
        </Stack>

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
    <Stack gap={2} width="100%">
      {!displayDetails ? (
        <Alert kind="danger">
          Cannot display the result from an unsupported check {reportOneof}.
        </Alert>
      ) : (
        <>
          {checkAttempt.status === diag.CheckAttemptStatus.ERROR && (
            <Alert kind="danger" details={<P2>{checkAttempt.error}</P2>}>
              Failed to {displayDetails.errorTitle}.
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
  }
> = {
  routeConflictReport: {
    errorTitle: 'inspect network routes',
    Component: CheckReportRouteConflict,
  },
};

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
          <Warning /> There{' '}
          {routeConflicts.length === 1
            ? 'is a network route'
            : 'are multiple network routes'}{' '}
          in conflict with VNet.
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
          { key: 'dest', headerText: 'Conflicting destination' },
          { key: 'vnetDest', headerText: 'VNet destination' },
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

const Summary = styled.summary`
  cursor: pointer;
`;

const Pre = styled.pre`
  white-space: pre-wrap;
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
  <DesignAlert m={0} width="100%" {...props} />
);
