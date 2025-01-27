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

import { ReactNode, useCallback } from 'react';

import { Alert, Flex, P2 } from 'design';
import { ActionButton } from 'design/Alert';
import { AlertProps } from 'design/Alert/Alert';
import {
  CheckAttemptStatus,
  CheckReportStatus,
} from 'gen-proto-ts/teleport/lib/vnet/diag/v1/diag_pb';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';

import { textSpacing } from './sliderStep';
import { useVnetContext } from './vnetContext';

export const DiagnosticsAlert = () => {
  const { diagnosticsAttempt, runDiagnostics, resetDiagnosticsAttempt } =
    useVnetContext();
  const { workspacesService } = useAppContext();
  const { close: closeConnectionsPanel } = useConnectionsContext();
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );
  const onDismiss = () => {
    // Reset the attempt, otherwise re-opening the panel would show the warning again.
    resetDiagnosticsAttempt();
  };

  if (
    diagnosticsAttempt.status === '' ||
    diagnosticsAttempt.status === 'processing'
  ) {
    return null;
  }

  if (diagnosticsAttempt.status === 'error') {
    return (
      <SliderStepAlert
        kind="danger"
        details={<P2>{diagnosticsAttempt.statusText}</P2>}
        onDismiss={onDismiss}
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
  const openReport = !rootClusterUri
    ? () => {}
    : () => {
        const docsService =
          workspacesService.getWorkspaceDocumentService(rootClusterUri);
        const doc = docsService.createVnetDiagReportDocument({
          rootClusterUri,
          report,
        });
        // TODO: Check if there's a doc with the same report, maybe add a UUID?
        docsService.add(doc);
        docsService.open(doc.uri);
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
    // TODO(ravicious): Once we start automatically running checks, this alert needs to be shown
    // only if the user manually requested diagnostics to be run. Alternatively, we can replace it
    // with some kind of a smaller "Everything's okay" indicator, but the user needs to be able to
    // open the report anyway.
    return (
      <SliderStepAlert
        kind="success"
        onDismiss={onDismiss}
        buttons={
          <ActionButton
            fill="border"
            intent="neutral"
            inputAlignment
            action={{ content: 'Open Report', onClick: openReport }}
            {...disabledOpenReportButtonProps}
          />
        }
      >
        No issues found.
      </SliderStepAlert>
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
      onDismiss={onDismiss}
      buttons={
        <>
          <ActionButton
            fill="border"
            intent="neutral"
            inputAlignment
            action={{ content: 'Open Report', onClick: openReport }}
            {...disabledOpenReportButtonProps}
          />
          <ActionButton
            fill="minimal"
            intent="neutral"
            inputAlignment
            action={{ content: 'Retry', onClick: runDiagnostics }}
          />
        </>
      }
    >
      {warningText}
    </SliderStepAlert>
  );
};

const SliderStepAlert = (
  props: (
    | { buttons?: ReactNode; details?: never }
    | { details?: ReactNode; buttons?: never }
  ) &
    Required<Pick<AlertProps, 'kind' | 'onDismiss' | 'children'>>
) => {
  const { buttons, details, ...alertProps } = props;

  return (
    <Alert
      mt={0}
      mx={textSpacing}
      mb={textSpacing}
      dismissible
      alignItems={buttons ? 'flex-start' : 'center'}
      details={
        buttons ? (
          <Flex mt={2} gap={2}>
            {buttons}
          </Flex>
        ) : (
          details
        )
      }
      {...alertProps}
    />
  );
};
