import React from 'react';
import { ButtonIcon, Flex, Text } from 'design';
import { CircleCross, CircleStop } from 'design/Icon';
import { TrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';

interface ConnectionItemProps {
  index: number;
  item: TrackedConnection;

  onActivate(): void;

  onRemove(): void;

  onDisconnect(): void;
}

export function ConnectionItem(props: ConnectionItemProps) {
  const offline = !props.item.connected;
  const color = !offline ? 'text.primary' : 'text.placeholder';
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
    <ListItem onClick={() => props.onActivate()} isActive={isActive}>
      <ConnectionStatusIndicator mr={2} connected={props.item.connected} />
      <Flex
        alignItems="center"
        justifyContent="space-between"
        flex="1"
        width="100%"
        minWidth="0"
      >
        <Text typography="body1" color={color} title={props.item.title}>
          {props.item.title}
        </Text>
        <ButtonIcon
          mr="-10px"
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
