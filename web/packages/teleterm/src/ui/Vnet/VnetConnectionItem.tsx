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

import React, {
  forwardRef,
  useCallback,
  useEffect,
  useMemo,
  useRef,
} from 'react';

import { Button, ButtonIcon, Flex, rotate360, Text } from 'design';
import * as icons from 'design/Icon';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { useStoreSelector } from 'teleterm/ui/hooks/useStoreSelector';
import { useConnectionsContext } from 'teleterm/ui/TopBar/Connections/connectionsContext';
import {
  Status as ConnectionStatus,
  ConnectionStatusIndicator,
} from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';

import { useVnetContext } from './vnetContext';

/**
 * VnetConnectionItem is the VNet entry in Connections.
 */
export const VnetConnectionItem = (props: {
  openVnetPanel: () => void;
  index: number;
  title: string;
}) => {
  const { isActive, scrollIntoViewIfActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.openVnetPanel,
  });

  const ref = useRef<HTMLLIElement>(null);

  useEffect(() => {
    scrollIntoViewIfActive(ref.current);
  }, [scrollIntoViewIfActive]);

  return (
    <VnetConnectionItemBase
      title="Open VNet panel"
      onClick={props.openVnetPanel}
      isActive={isActive}
      ref={ref}
    />
  );
};

export const VnetSliderStepHeader = (props: {
  goBack: () => void;
  runDiagnosticsFromVnetPanel: () => Promise<unknown>;
}) => (
  <VnetConnectionItemBase
    title="Go back to Connections"
    onClick={props.goBack}
    showBackButton
    showExtraRightButtons
    // Make the element focusable.
    tabIndex={0}
    runDiagnosticsFromVnetPanel={props.runDiagnosticsFromVnetPanel}
  />
);

const VnetConnectionItemBase = forwardRef<
  HTMLLIElement,
  {
    onClick: () => void;
    title: string;
    showBackButton?: boolean;
    /**
     * Shows help and diagnostics buttons between "VNet" text and the start/stop button.
     * Also adds text to the VNet toggle button rather than using just an icon.
     */
    showExtraRightButtons?: boolean;
    isActive?: boolean;
    tabIndex?: number;
  } & (
    | { showExtraRightButtons?: false }
    | {
        showExtraRightButtons: true;
        runDiagnosticsFromVnetPanel: () => Promise<unknown>;
      }
  )
>((props, ref) => {
  const { workspacesService } = useAppContext();
  const {
    status,
    start,
    stop,
    startAttempt,
    stopAttempt,
    diagnosticsAttempt,
    getDisabledDiagnosticsReason,
    showDiagWarningIndicator,
  } = useVnetContext();
  const { close: closeConnectionsPanel } = useConnectionsContext();
  const rootClusterUri = useStoreSelector(
    'workspacesService',
    useCallback(state => state.rootClusterUri, [])
  );
  const isUserInWorkspace = !!rootClusterUri;
  const isProcessing =
    startAttempt.status === 'processing' || stopAttempt.status === 'processing';
  const disabledDiagnosticsReason =
    getDisabledDiagnosticsReason(diagnosticsAttempt);
  const indicatorStatus: ConnectionStatus = useMemo(() => {
    // Consider an error state first. If there was an error, status.value is not 'running'.
    if (
      startAttempt.status === 'error' ||
      stopAttempt.status === 'error' ||
      (status.value === 'stopped' &&
        status.reason.value === 'unexpected-shutdown')
    ) {
      return 'error';
    }

    if (status.value === 'stopped') {
      return 'off';
    }

    if (showDiagWarningIndicator) {
      return 'warning';
    }

    return 'on';
  }, [startAttempt, stopAttempt, status, showDiagWarningIndicator]);

  const onEnterPress = (event: React.KeyboardEvent) => {
    if (
      event.key !== 'Enter' ||
      // onKeyDown propagates from children too.
      // Ignore those events, handle only keypresses on ListItem.
      event.target !== event.currentTarget
    ) {
      return;
    }

    props.onClick();
  };

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
    closeConnectionsPanel();
  };

  return (
    <ListItem
      ref={ref}
      css={`
        padding: ${props => props.theme.space[1]}px
          ${props => props.theme.space[2]}px;
        height: unset;
      `}
      isActive={props.isActive}
      title={props.title}
      onClick={props.onClick}
      onKeyDown={onEnterPress}
      tabIndex={props.tabIndex}
    >
      {props.showBackButton ? (
        <icons.ArrowBack size="small" mr={2} />
      ) : (
        <ConnectionStatusIndicator
          mr={3}
          css={`
            flex-shrink: 0;
          `}
          status={indicatorStatus}
        />
      )}
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
          {props.showExtraRightButtons && (
            <>
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
            </>
          )}

          {/* The conditions for the buttons below could be written in a more concise way.
                However, what's important for us here is that React keeps focus on the same
                "logical" button when this component transitions between different states.
                As a result, we cannot e.g. use a fragment to group two different states together.

                There's a test which checks whether the focus is kept between state transitions.
            */}

          {isProcessing &&
            // This button cannot be disabled, otherwise the focus will be lost between
            // transitions and the test won't be able to catch this.
            (props.showExtraRightButtons ? (
              <Button
                key={toggleVnetButtonKey}
                width={toggleVnetButtonWidth}
                size="small"
                intent="neutral"
                fill="minimal"
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
            ) : (
              <ButtonIcon
                key={toggleVnetButtonKey}
                title={
                  status.value === 'running' ? 'Stopping VNet' : 'Starting VNet'
                }
                onClick={e => {
                  e.stopPropagation();
                }}
              >
                <icons.Spinner
                  css={`
                    width: 32px;
                    height: 32px;
                    animation: ${rotate360} 1.5s infinite linear;
                  `}
                  size={18}
                />
              </ButtonIcon>
            ))}
          {!isProcessing &&
            status.value === 'running' &&
            (props.showExtraRightButtons ? (
              <Button
                intent="neutral"
                fill="minimal"
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
            ) : (
              <ButtonIcon
                key={toggleVnetButtonKey}
                title="Stop VNet"
                onClick={e => {
                  e.stopPropagation();
                  stop();
                }}
              >
                <icons.BroadcastSlash size={18} />
              </ButtonIcon>
            ))}
          {!isProcessing &&
            status.value === 'stopped' &&
            (props.showExtraRightButtons ? (
              <Button
                key={toggleVnetButtonKey}
                size="small"
                width={toggleVnetButtonWidth}
                title=""
                onClick={e => {
                  e.stopPropagation();
                  start();
                }}
              >
                <icons.Broadcast size={18} mr={1} /> Start VNet
              </Button>
            ) : (
              <ButtonIcon
                key={toggleVnetButtonKey}
                title="Start VNet"
                onClick={e => {
                  e.stopPropagation();
                  start();
                }}
              >
                <icons.Broadcast size={18} />
              </ButtonIcon>
            ))}
        </Flex>
      </Flex>
    </ListItem>
  );
});

const toggleVnetButtonKey = 'vnet-toggle';
const toggleVnetButtonWidth = 102;
