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

import { PropsWithChildren, ReactNode, useCallback } from 'react';

import { Alert, Flex, P2 } from 'design';
import { ActionButton } from 'design/Alert';
import { AlertKind } from 'design/Alert/Alert';
import { Checks } from 'design/Icon';
import { StatusIcon } from 'design/StatusIcon';
import {
  CheckAttemptStatus,
  CheckReportStatus,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { textSpacing } from './sliderStep';
import { useVnetContext } from './vnetContext';

export const DiagnosticsAlert = (props: {
  runDiagnosticsFromVnetPanel: () => Promise<unknown>;
}) => {
  const {
    diagnosticsAttempt,
    dismissDiagnosticsAlert,
    hasDismissedDiagnosticsAlert,
  } = useVnetContext();
  const { workspacesService } = useAppContext();
  const { close: closeConnectionsPanel } = useConnectionsContext();
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );

  if (
    diagnosticsAttempt.status === '' ||
    // If diagnostics are currently running, but there are results from the previous run, display
    // the results from the previous run. Otherwise display nothing.
    (diagnosticsAttempt.status === 'processing' && !diagnosticsAttempt.data) ||
    hasDismissedDiagnosticsAlert
  ) {
    return null;
  }

  if (diagnosticsAttempt.status === 'error') {
    return (
      <SliderStepAlert
        kind="danger"
        details={<P2>{diagnosticsAttempt.statusText}</P2>}
      >
        Encountered an error while running diagnostics
      </SliderStepAlert>
    );
  }

  const report = diagnosticsAttempt.data;
  const disabledOpenReportButtonProps = !rootClusterUri
    ? {
        disabled: true,
        title: 'Log in to a cluster to see the full report',
      }
    : {};
  const openReport = () => {
    if (!rootClusterUri) {
      return;
    }

    const docsService =
      workspacesService.getWorkspaceDocumentService(rootClusterUri);

    // Check for an existing doc first. It may be present if someone re-runs diagnostics from within
    // a doc, then opens the VNet panel and clicks "Open Diag Report". The report in the panel and
    // the report in the doc are equal in that case, as they both come from diagnosticsAttempt.data.
    const existingDoc = docsService.getDocuments().find(
      d =>
        d.kind === 'doc.vnet_diag_report' &&
        // Reports don't have IDs, so createdAt is used as a good-enough approximation of an ID.
        d.report?.createdAt === report.createdAt
    );
    if (existingDoc) {
      docsService.open(existingDoc.uri);
    } else {
      const doc = docsService.createVnetDiagReportDocument({
        rootClusterUri,
        report,
      });
      docsService.add(doc);
      docsService.open(doc.uri);
    }
    closeConnectionsPanel();
  };

  if (
    report.checks.length &&
    report.checks.every(
      checkAttempt =>
        checkAttempt.status === CheckAttemptStatus.OK &&
        checkAttempt.checkReport.status === CheckReportStatus.OK
    )
  ) {
    return (
      <Flex px={textSpacing} justifyContent="space-between" alignItems="center">
        <Flex gap={1}>
          <StatusIcon kind="neutral" customIcon={Checks} size="large" />
          No issues found.
        </Flex>

        <ActionButton
          fill="minimal"
          intent="neutral"
          inputAlignment
          action={{ content: 'Open Diag Report', onClick: openReport }}
          {...disabledOpenReportButtonProps}
        />
      </Flex>
    );
  }

  // If this default warningText is shown the user, it means we failed to account for a specific
  // state.
  let warningText = 'Unknown report status';
  if (
    report.checks.some(
      checkAttempt =>
        checkAttempt.status === CheckAttemptStatus.OK &&
        checkAttempt.checkReport.status === CheckReportStatus.ISSUES_FOUND
    )
  ) {
    warningText = 'Other software on your device might interfere with VNet.';
  } else if (
    report.checks.some(
      checkAttempt => checkAttempt.status === CheckAttemptStatus.ERROR
    )
  ) {
    warningText = 'Some diagnostic checks failed to report results.';
  }

  return (
    <SliderStepAlert
      kind="warning"
      onDismiss={dismissDiagnosticsAlert}
      buttons={
        <>
          <ActionButton
            fill="border"
            intent="neutral"
            inputAlignment
            action={{ content: 'Open Diag Report', onClick: openReport }}
            {...disabledOpenReportButtonProps}
          />
          <ActionButton
            fill="minimal"
            intent="neutral"
            inputAlignment
            action={{
              content: 'Retry',
              onClick: props.runDiagnosticsFromVnetPanel,
            }}
          />
        </>
      }
    >
      {warningText}
    </SliderStepAlert>
  );
};

const SliderStepAlert = (
  props: PropsWithChildren<{
    kind: AlertKind;
    onDismiss?: () => void;
  }> &
    (
      | { buttons?: ReactNode; details?: never }
      | { details?: ReactNode; buttons?: never }
    )
) => {
  const { buttons, onDismiss } = props;

  return (
    <Alert
      kind={props.kind}
      mt={0}
      mx={textSpacing}
      mb={textSpacing}
      {...(onDismiss ? { dismissible: true, onDismiss } : {})}
      alignItems={buttons ? 'flex-start' : 'center'}
      details={
        buttons ? (
          <Flex mt={2} gap={2}>
            {buttons}
          </Flex>
        ) : (
          props.details
        )
      }
    >
      {props.children}
    </Alert>
  );
};
