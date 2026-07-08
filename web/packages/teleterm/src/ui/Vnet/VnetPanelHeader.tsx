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

import { useCallback, useMemo } from 'react';

import { Button, ButtonIcon, Flex, Text } from 'design';
import * as icons from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import {
  Status as ConnectionStatus,
  ConnectionStatusIndicator,
} from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { useVnetContext } from './vnetContext';

/**
 * VnetPanelHeader is the top row of the VNet panel. It shows the VNet status
 * indicator, the "VNet" label, the help and diagnostics buttons and the
 * start/stop toggle.
 */
export const VnetPanelHeader = (props: {
  runDiagnosticsFromVnetPanel: () => Promise<unknown>;
}) => {
  const { workspacesService } = useAppContext();
  const {
    status,
    start,
    stop,
    startAttempt,
    stopAttempt,
    diagnosticsAttempt,
    getDisabledDiagnosticsReason,
    installTimeRequirementsCheck,
    closePanel,
  } = useVnetContext();
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );
  const isUserInWorkspace = !!rootClusterUri;
  const isProcessing =
    startAttempt.status === 'processing' ||
    stopAttempt.status === 'processing' ||
    installTimeRequirementsCheck.status === 'unknown';
  const disabledDiagnosticsReason =
    getDisabledDiagnosticsReason(diagnosticsAttempt);
  const indicatorStatus: ConnectionStatus = useMemo(() => {
    // Consider an error state first. If there was an error, status.value is not 'running'.
    if (
      startAttempt.status === 'error' ||
      stopAttempt.status === 'error' ||
      installTimeRequirementsCheck.status === 'failed' ||
      (status.value === 'stopped' &&
        status.reason.value === 'unexpected-shutdown')
    ) {
      return 'error';
    }

    if (status.value === 'stopped') {
      return 'off';
    }

    return 'on';
  }, [startAttempt, stopAttempt, status, installTimeRequirementsCheck]);

  const openDocumentVnetInfo = () => {
    if (!rootClusterUri) {
      return;
    }

    const docsService =
      workspacesService.getWorkspaceDocumentService(rootClusterUri);

    docsService.openExistingOrAddNew(
      d => d.kind === 'doc.vnet_info',
      () => docsService.createVnetInfoDocument({ rootClusterUri })
    );
    closePanel();
  };

  return (
    <Flex
      alignItems="center"
      css={`
        padding: ${props => props.theme.space[1]}px
          ${props => props.theme.space[2]}px;
        height: unset;
        cursor: default;
      `}
      // Make the element focusable so that the next tab press focuses the header buttons.
      tabIndex={0}
      // The header itself is not clickable, it only groups the controls.
      onClick={() => {}}
    >
      <ConnectionStatusIndicator
        mr={3}
        css={`
          flex-shrink: 0;
        `}
        status={indicatorStatus}
      />
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        minWidth="0"
      >
        <div
          css={`
            min-width: 0;
          `}
        >
          <Text
            typography="body2"
            bold
            color="text.main"
            css={`
              line-height: 16px;
            `}
          >
            VNet
          </Text>
          <Text color="text.slightlyMuted" typography="body3">
            Virtual Network Emulation
          </Text>
        </div>

        {/* Buttons to the right. Negative margin to match buttons of other connections. */}
        <Flex gap={1} mr="-3px">
          {isUserInWorkspace ? (
            <ButtonIcon
              title="Open information about VNet"
              onClick={e => {
                // Don't trigger ListItem's onClick.
                e.stopPropagation();
                openDocumentVnetInfo();
              }}
            >
              <icons.Question size={18} />
            </ButtonIcon>
          ) : (
            // If the user is not logged in to any workspace, a new doc cannot be opened.
            // Instead, show a link to the documentation.
            <ButtonIcon
              as="a"
              title="Open VNet documentation"
              href="https://goteleport.com/docs/connect-your-client/vnet/"
              target="_blank"
              onClick={e => {
                // Don't trigger ListItem's onClick.
                e.stopPropagation();
              }}
            >
              <icons.Question size={18} />
            </ButtonIcon>
          )}

          <ButtonIcon
            title={disabledDiagnosticsReason || 'Run diagnostics'}
            disabled={!!disabledDiagnosticsReason}
            onClick={e => {
              e.stopPropagation();
              props.runDiagnosticsFromVnetPanel();
            }}
          >
            <icons.ListMagnifyingGlass size={18} />
          </ButtonIcon>

          {/* The conditions for the buttons below could be written in a more concise way.
                However, what's important for us here is that React keeps focus on the same
                "logical" button when this component transitions between different states.
                As a result, we cannot e.g. use a fragment to group two different states together.

                There's a test which checks whether the focus is kept between state transitions.
            */}

          {isProcessing && (
            // This button cannot be disabled, otherwise the focus will be lost between
            // transitions and the test won't be able to catch this.
            <Button
              key={toggleVnetButtonKey}
              width={toggleVnetButtonWidth}
              size="small"
              intent="neutral"
              fill="filled"
              title=""
              onClick={e => {
                e.stopPropagation();
              }}
            >
              {status.value === 'running' ? (
                <>
                  <icons.BroadcastSlash size={18} mr={1} /> Stopping…
                </>
              ) : (
                <>
                  <icons.Broadcast size={18} mr={1} /> Starting…
                </>
              )}
            </Button>
          )}
          {!isProcessing && status.value === 'running' && (
            <Button
              intent="neutral"
              fill="filled"
              key={toggleVnetButtonKey}
              size="small"
              width={toggleVnetButtonWidth}
              title=""
              onClick={e => {
                e.stopPropagation();
                stop();
              }}
            >
              <icons.BroadcastSlash size={18} mr={1} />
              Stop VNet
            </Button>
          )}
          {!isProcessing && status.value === 'stopped' && (
            <Button
              key={toggleVnetButtonKey}
              size="small"
              width={toggleVnetButtonWidth}
              disabled={installTimeRequirementsCheck.status === 'failed'}
              title=""
              onClick={e => {
                e.stopPropagation();
                start();
              }}
            >
              <icons.Broadcast size={18} mr={1} /> Start VNet
            </Button>
          )}
        </Flex>
      </Flex>
    </Flex>
  );
};

const toggleVnetButtonKey = 'vnet-toggle';
const toggleVnetButtonWidth = 102;
