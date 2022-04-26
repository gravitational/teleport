import React from 'react';
import { ButtonIcon, Flex, Text } from 'design';
import { CircleCross, CircleStop } from 'design/Icon';
import { ExtendedTrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

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
      Icon: CircleStop,
    },
    remove: {
      title: 'Remove',
      action: props.onRemove,
      Icon: CircleCross,
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
            {props.item.title}
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
          color="text.placeholder"
          title={actionIcon.title}
          onClick={e => {
            e.stopPropagation();
            actionIcon.action();
          }}
        >
          <actionIcon.Icon fontSize={12} />
        </ButtonIcon>
      </Flex>
    </ListItem>
  );
}
