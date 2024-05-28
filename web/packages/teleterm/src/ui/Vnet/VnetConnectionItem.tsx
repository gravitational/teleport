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

import React, { forwardRef, useEffect, useRef } from 'react';
import styled from 'styled-components';
import { Text, ButtonIcon, Flex, rotate360 } from 'design';
import * as icons from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';

import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';
import { ListItem, StaticListItem } from 'teleterm/ui/components/ListItem';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { useAppContext } from 'teleterm/ui/appContextProvider';

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

  const ref = useRef<HTMLElement>();

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

export const VnetSliderStepHeader = (props: { goBack: () => void }) => (
  <VnetConnectionItemBase
    title="Go back to Connections"
    onClick={props.goBack}
    showBackButton
    showHelpButton
    // Make the element focusable.
    tabIndex={0}
  />
);

const VnetConnectionItemBase = forwardRef(
  (
    props: {
      onClick: () => void;
      title: string;
      showBackButton?: boolean;
      showHelpButton?: boolean;
      isActive?: boolean;
      tabIndex?: number;
    },
    ref
  ) => {
    const { status, start, stop, startAttempt, stopAttempt } = useVnetContext();
    const isProcessing =
      startAttempt.status === 'processing' ||
      stopAttempt.status === 'processing';
    const indicatorStatus =
      startAttempt.status === 'error' || stopAttempt.status === 'error'
        ? 'error'
        : status === 'running'
          ? 'on'
          : 'off';

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
              typography="body1"
              bold
              color="text.main"
              css={`
                line-height: 16px;
              `}
            >
              VNet
            </Text>
            <Text color="text.slightlyMuted" typography="body2">
              Virtual Network Emulation
            </Text>
          </div>

          {/* Buttons to the right. Negative margin to match buttons of other connections. */}
          <Flex gap={1} mr="-3px">
            {props.showHelpButton && (
              <ButtonIcon
                as="a"
                title="Open VNet documentation"
                href="https://goteleport.com/docs/connect-your-client/teleport-connect/#vnet"
                target="_blank"
                onClick={e => {
                  // Don't trigger ListItem's onClick.
                  e.stopPropagation();
                }}
              >
                <icons.Question size={18} />
              </ButtonIcon>
            )}

            {/* The conditions for the buttons below could be written in a more concise way.
                However, what's important for us here is that React keeps focus on the same
                "logical" button when this component transitions between different states.
                As a result, we cannot e.g. use a fragment to group two different states together.

                There's a test which checks whether the focus is kept between state transitions.
            */}

            {isProcessing && (
              // This button cannot be disabled, otherwise the focus will be lost between
              // transitions and the test won't be able to catch this.
              <ButtonIcon
                key="vnet-toggle"
                title={status === 'stopped' ? 'Starting VNet' : 'Stopping VNet'}
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
            )}
            {!isProcessing && status === 'running' && (
              <ButtonIcon
                key="vnet-toggle"
                title="Stop VNet"
                onClick={e => {
                  e.stopPropagation();
                  stop();
                }}
              >
                <icons.BroadcastSlash size={18} />
              </ButtonIcon>
            )}
            {!isProcessing && status === 'stopped' && (
              <ButtonIcon
                key="vnet-toggle"
                title="Start VNet"
                onClick={e => {
                  e.stopPropagation();
                  start();
                }}
              >
                <icons.Broadcast size={18} />
              </ButtonIcon>
            )}
          </Flex>
        </Flex>
      </ListItem>
    );
  }
);

/**
 * AppConnectionItem is an individual connection to an app made through VNet, shown in
 * VnetSliderStep.
 */
export const AppConnectionItem = (props: {
  app: string;
  status: 'on' | 'error' | 'off';
  // TODO(ravicious): Refactor the status type so that the error prop is available only if status is
  // set to 'error'.
  error?: string;
}) => {
  const { notificationsService } = useAppContext();

  const copy = async () => {
    const content = [props.app, props.error].filter(Boolean).join(': ');
    await copyToClipboard(content);

    notificationsService.notifyInfo(
      props.error
        ? `Copied error for ${props.app} to clipboard`
        : `Copied ${props.app} to clipboard`
    );
  };

  return (
    <StaticListItem
      title={props.app}
      as="div"
      css={`
        padding: 0 ${props => props.theme.space[2]}px;
        height: unset;
      `}
    >
      <ConnectionStatusIndicator
        mr={3}
        css={`
          flex-shrink: 0;
        `}
        status={props.status}
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
            typography="body1"
            color="text.main"
            css={`
              line-height: 16px;
            `}
          >
            {props.app}
          </Text>
          {props.error && (
            <Text
              color="text.slightlyMuted"
              typography="body2"
              title={props.error}
            >
              {props.error}
            </Text>
          )}
        </div>

        {/* Button to the right. */}
        <ButtonIconOnHover onClick={copy} title="Copy to clipboard">
          <icons.Clipboard size={18} />
        </ButtonIconOnHover>
      </Flex>
    </StaticListItem>
  );
};

const ButtonIconOnHover = styled(ButtonIcon)`
  ${StaticListItem}:not(:hover) & {
    visibility: hidden;
    // Disable transition so that the button shows up immediately on hover, but still retains the
    // original transition value once visible.
    transition: none;
  }
`;
