/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useEffect, useRef } from 'react';
import { ButtonIcon, Flex, Text } from 'design';
import { Trash, Unlink } from 'design/Icon';

import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ListItem } from 'teleterm/ui/components/ListItem';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { isAppUri, isDatabaseUri } from 'teleterm/ui/uri';

import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';

interface ConnectionItemProps {
  index: number;
  item: ExtendedTrackedConnection;

  onActivate(): void;

  onRemove(): void;

  onDisconnect(): void;
}

export function ConnectionItem(props: ConnectionItemProps) {
  const offline = !props.item.connected;
  const { isActive, scrollIntoViewIfActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onActivate,
  });

  const actionIcons = {
    disconnect: {
      title: 'Disconnect',
      action: props.onDisconnect,
      Icon: Unlink,
    },
    remove: {
      title: 'Remove',
      action: props.onRemove,
      Icon: Trash,
    },
  };

  const actionIcon = offline ? actionIcons.remove : actionIcons.disconnect;
  const ref = useRef<HTMLElement>();

  useEffect(() => {
    scrollIntoViewIfActive(ref.current);
  }, [scrollIntoViewIfActive]);

  return (
    <ListItem
      onClick={props.onActivate}
      isActive={isActive}
      ref={ref}
      css={`
        padding: 6px 8px;
        height: unset;
      `}
    >
      <ConnectionStatusIndicator
        mr={3}
        css={`
          flex-shrink: 0;
        `}
        status={props.item.connected ? 'on' : 'off'}
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
            bold
            color="text.main"
            title={props.item.title}
            css={`
              line-height: 16px;
            `}
          >
            <span
              css={`
                font-size: 10px;
                background: ${props => props.theme.colors.spotBackground[2]};
                opacity: 0.85;
                padding: 1px 2px;
                margin-right: 4px;
                border-radius: 4px;
              `}
            >
              {getKindName(props.item)}
            </span>
            <span
              css={`
                vertical-align: middle;
              `}
            >
              {props.item.title}
            </span>
          </Text>
          <Text
            color="text.slightlyMuted"
            typography="body2"
            title={props.item.clusterName}
          >
            {props.item.clusterName}
          </Text>
        </div>
        <ButtonIcon
          mr="-3px"
          title={actionIcon.title}
          onClick={e => {
            e.stopPropagation();
            actionIcon.action();
          }}
        >
          <actionIcon.Icon size={18} />
        </ButtonIcon>
      </Flex>
    </ListItem>
  );
}

function getKindName(connection: ExtendedTrackedConnection): string {
  switch (connection.kind) {
    case 'connection.gateway':
      if (isAppUri(connection.targetUri)) {
        return 'APP';
      }
      if (isDatabaseUri(connection.targetUri)) {
        return 'DB';
      }
      return 'UNKNOWN';
    case 'connection.server':
      return 'SSH';
    case 'connection.kube':
      return 'KUBE';
    default:
      // The default branch is triggered when the state read from the disk
      // contains a connection not supported by the given Connect version.
      //
      // For example, the user can open an app connection in Connect v15
      // and then downgrade to a version that doesn't support apps.
      // That connection should be shown as 'UNKNOWN' in the connection list.
      return 'UNKNOWN';
  }
}
