import React from 'react';
import { ButtonIcon, Flex, Text } from 'design';
import { Trash, Unlink } from 'design/Icon';

import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { assertUnreachable } from 'teleterm/ui/utils';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

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
  const { isActive } = useKeyboardArrowsNavigation({
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

  return (
    <ListItem
      onClick={props.onActivate}
      isActive={isActive}
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
        connected={props.item.connected}
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
            color="text.primary"
            title={props.item.title}
            css={`
              line-height: 16px;
            `}
          >
            <span
              css={`
                font-size: 10px;
                background: rgba(255, 255, 255, 0.25);
                opacity: 0.85;
                padding: 1px 2px;
                margin-right: 4px;
                border-radius: 4px;
              `}
            >
              {getKindName(props.item.kind)}
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
            color="text.secondary"
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
          <actionIcon.Icon fontSize={13} />
        </ButtonIcon>
      </Flex>
    </ListItem>
  );
}

function getKindName(kind: ExtendedTrackedConnection['kind']): string {
  switch (kind) {
    case 'connection.gateway':
      return 'DB';
    case 'connection.server':
      return 'SSH';
    case 'connection.kube':
      return 'KUBE';
    default:
      assertUnreachable(kind);
  }
}
