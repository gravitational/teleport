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

import { Text, ButtonIcon, Flex, rotate360 } from 'design';
import * as icons from 'design/Icon';
import { copyToClipboard } from 'design/utils/copyToClipboard';

import { ConnectionStatusIndicator } from 'teleterm/ui/TopBar/Connections/ConnectionsFilterableList/ConnectionStatusIndicator';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { useAppContext } from 'teleterm/ui/appContextProvider';

import { useVnetContext } from './vnetContext';

/**
 * VnetConnection is the Vnet entry in Connections. It is also used as the header in VnetSliderStep.
 */
export const VnetConnectionItem = (props: {
  onClick: () => void;
  title: string;
  showBackButton?: boolean;
}) => {
  const { status, start, stop, startAttempt, stopAttempt } = useVnetContext();

  return (
    <ListItem
      css={`
        padding: 6px 8px;
        ${props.showBackButton ? 'padding-left: 0;' : ''}
        height: unset;
      `}
      onClick={props.onClick}
      title={props.title}
    >
      {props.showBackButton ? (
        <icons.ArrowBack size="small" mr={3} />
      ) : (
        <ConnectionStatusIndicator
          mr={3}
          css={`
            flex-shrink: 0;
          `}
          status={
            startAttempt.status === 'error' || stopAttempt.status === 'error'
              ? 'error'
              : status === 'running'
                ? 'on'
                : 'off'
          }
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
            <span
              css={`
                vertical-align: middle;
              `}
            >
              VNet
            </span>
          </Text>
        </div>

        {/* Button to the right. */}
        {startAttempt.status === 'processing' ||
        stopAttempt.status === 'processing' ? (
          <icons.Spinner
            css={`
              width: 32px;
              height: 32px;
              animation: ${rotate360} 1.5s infinite linear;
            `}
            mr="-3px"
            size={18}
          />
        ) : (
          <>
            {status === 'running' && (
              <ButtonIcon
                mr="-3px"
                title="Stop VNet"
                onClick={e => {
                  // Don't trigger onClick that's on ListItem.
                  e.stopPropagation();
                  stop();
                }}
              >
                <icons.Unlink size={18} />
              </ButtonIcon>
            )}
            {status === 'stopped' && (
              <ButtonIcon
                mr="-3px"
                title="Start VNet"
                onClick={e => {
                  e.stopPropagation();
                  start();
                }}
              >
                <icons.Link size={18} />
              </ButtonIcon>
            )}
          </>
        )}
      </Flex>
    </ListItem>
  );
};

/**
 * AppConnectionItem is an individual connection to an app made through VNet, shown in
 * VnetSliderStep.
 */
export const AppConnectionItem = (props: {
  app: string;
  status: 'on' | 'error' | 'off';
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
    <ListItem
      css={`
        padding: 6px 8px;
        height: unset;
      `}
      onClick={copy}
      title="Copy to clipboard"
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
            <span
              css={`
                vertical-align: middle;
              `}
            >
              {props.app}
            </span>
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
      </Flex>
    </ListItem>
  );
};
