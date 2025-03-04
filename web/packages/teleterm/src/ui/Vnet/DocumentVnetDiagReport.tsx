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

import {
  Box,
  Button,
  ButtonPrimary,
  Alert as DesignAlert,
  Flex,
  H1,
  Link,
  ResponsiveImage,
  Stack,
} from 'design';
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
import { H2, H3, P1, P2 } from 'design/Text/Text';
import { HoverTooltip } from 'design/Tooltip';
import { copyToClipboard } from 'design/utils/copyToClipboard';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import * as diag from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';
import { CanceledError, useAsync } from 'shared/hooks/useAsync';
import { pluralize } from 'shared/utils/text';

import { reportOneOfIsRouteConflictReport } from 'teleterm/helpers';
import { proxyHostname } from 'teleterm/services/tshd/cluster';
import { getReportFilename, reportToText } from 'teleterm/services/vnet/diag';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import Document from 'teleterm/ui/Document';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import type * as docTypes from 'teleterm/ui/services/workspacesService';

import imgNoVnetCurl from './no-vnet-curl.png';
import imgNoVnetProxy from './no-vnet-proxy.png';
import svgWebAppWithoutVnet from './recording-proxy.svg';
import svgWebAppVnet from './session-recording.svg';
import imgVnetCurl from './vnet-curl.png';
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
  const { documentsService, rootClusterUri } = useWorkspaceContext();
  const rootCluster = useStoreSelector(
    'clustersService',
    useCallback(state => state.clusters.get(rootClusterUri), [rootClusterUri])
  );

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

  return (
    <Document visible={props.visible}>
      <Stack
        gap={4}
        maxWidth="1360px"
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
        <Flex width="100%" gap={5} flexWrap="wrap">
          <Stack gap={2}>
            <H1>Teleport VNet</H1>

            <P1
              css={`
                max-width: 60ch;
              `}
            >
              VNet automatically proxies connections from your computer to TCP
              apps available through Teleport. Any program on your device can
              connect to an application behind Teleport with no extra steps.
              Underneath, VNet authenticates the connection with your
              credentials. This is all done client-side – VNet sets up a local
              DNS name server, a virtual network device, and a proxy.
            </P1>
            <P1 m={0}>VNet makes it easy to connect to…</P1>
          </Stack>

          <Flex
            flex={1}
            alignItems="center"
            justifyContent="center"
            // Make sure the text in the button doesn't ever break into two lines.
            minWidth="fit-content"
          >
            <Stack gap={2} alignItems="center">
              <Button size="extra-large">Start VNet</Button>
              <Button
                fill="minimal"
                intent="neutral"
                as="a"
                href="https://goteleport.com/docs/connect-your-client/vnet/"
                target="_blank"
                inputAlignment
              >
                Open Documentation
              </Button>
            </Stack>
          </Flex>
        </Flex>

        <Stack gap={5}>
          <Stack gap={3} width="100%">
            <Flex
              width="100%"
              alignItems="baseline"
              gap={4}
              flexWrap="wrap"
              justifyContent="space-between"
            >
              <H2>TCP APIs</H2>

              <Button
                fill="minimal"
                intent="neutral"
                as="a"
                href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/"
                target="_blank"
                inputAlignment
              >
                Learn More
              </Button>
            </Flex>

            <Flex width="100%" gap={4} flexWrap="wrap">
              <Stack
                flex={1}
                p={3}
                gap={2}
                borderRadius={3}
                minWidth="400px"
                maxWidth="600px"
                backgroundColor="levels.elevated"
                borderColor="levels.popout"
                borderWidth={2}
                borderStyle="solid"
              >
                <Stack>
                  <H3>With VNet</H3>
                  {/*
                <P2>No local proxy needed – connect directly to the app.</P2>
                  */}
                  <P2>Connect directly to the app.</P2>
                </Stack>

                <ResponsiveImage src={imgVnetCurl} alt="curl call with VNet" />
              </Stack>

              <Stack
                flex={2}
                p={3}
                gap={2}
                borderRadius={3}
                minWidth="800px"
                // backgroundColor="levels.elevated"
                borderColor="levels.popout"
                borderWidth={2}
                borderStyle="dashed"
              >
                <Stack>
                  <H3>Without VNet</H3>
                  <P2>
                    A local proxy has to be set up first. The client program
                    needs to connect over localhost on the correct port.
                  </P2>
                </Stack>

                <Flex width="100%" gap={3}>
                  <div>
                    <ResponsiveImage src={imgNoVnetProxy} alt="tsh proxy app" />
                  </div>
                  <div>
                    <ResponsiveImage
                      src={imgNoVnetCurl}
                      alt="curl call without VNet"
                    />
                  </div>
                </Flex>
              </Stack>
            </Flex>
          </Stack>

          <Stack gap={3} width="100%">
            <Flex
              width="100%"
              alignItems="baseline"
              gap={4}
              flexWrap="wrap"
              justifyContent="space-between"
            >
              <H2>Web Applications With 3rd-Party SSO</H2>

              <Button
                fill="minimal"
                intent="neutral"
                as="a"
                href="https://goteleport.com/docs/enroll-resources/application-access/guides/vnet/#accessing-web-apps-through-vnet"
                target="_blank"
                inputAlignment
              >
                Learn More
              </Button>
            </Flex>

            <Flex width="100%" gap={4} flexWrap="wrap">
              <Box
                p={3}
                flex={1}
                borderRadius={3}
                backgroundColor="levels.elevated"
                borderColor="levels.popout"
                borderWidth={2}
                borderStyle="solid"
                css={`
                  display: grid;
                  gap: ${props => props.theme.space[2]}px;
                  grid-auto-flow: row;
                  grid-auto-columns: 1fr;
                `}
              >
                <Stack>
                  <H3>With VNet</H3>
                  <P2>
                    The app is protected from unauthenticated traffic by
                    Teleport in a way that is transparent to&nbsp;VNet users.
                    It's accessible under the same domain with no changes to the
                    SSO setup.
                  </P2>
                </Stack>

                <Box flex={1} backgroundColor="white" px={2} py={3}>
                  <ResponsiveImage
                    alt="Web app with VNet"
                    src={svgWebAppVnet}
                  />
                </Box>
              </Box>

              <Box
                p={3}
                flex={1}
                borderRadius={3}
                // backgroundColor="levels.elevated"
                borderColor="levels.popout"
                borderWidth={2}
                borderStyle="dashed"
                css={`
                  display: grid;
                  gap: ${props => props.theme.space[2]}px;
                  grid-auto-flow: row;
                  grid-auto-columns: 1fr;
                `}
              >
                <Stack>
                  <H3>Without VNet</H3>
                  {/*
                  <P2>
                    Access to the app is gated by both Teleport Proxy Service
                    and 3rd-party SSO. The app is now accessible under the
                    domain of the Proxy Service, so SSO redirect URLs need to be
                    updated.
                  </P2>
                  */}
                  <P2>
                    Either the app accepts Internet traffic and is protected
                    only by SSO or it is behind Teleport, so admins have to
                    update redirect URLs and users authenticate with both
                    Teleport and SSO.
                  </P2>
                </Stack>

                <Box backgroundColor="white" px={2} py={3}>
                  <ResponsiveImage
                    alt="Web app without VNet"
                    src={svgWebAppWithoutVnet}
                  />
                </Box>
              </Box>
            </Flex>
          </Stack>
        </Stack>
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
          Cannot display the result from an unsupported check {reportOneof}
        </Alert>
      ) : (
        <>
          {checkAttempt.status === diag.CheckAttemptStatus.ERROR && (
            <Alert kind="danger" details={<P2>{checkAttempt.error}</P2>}>
              Failed to {displayDetails.errorTitle}
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
