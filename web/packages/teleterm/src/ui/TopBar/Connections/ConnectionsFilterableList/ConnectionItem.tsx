import React from 'react';
import { ButtonIcon, Flex, Text } from 'design';
import { CircleStop, CircleCross } from 'design/Icon';
import { TrackedConnection } from 'teleterm/ui/services/connectionTracker';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { ConnectionStatusIndicator } from './ConnectionStatusIndicator';

interface ConnectionItemProps {
  item: TrackedConnection;

  onActivate(id: string): void;

  onRemove(id: string): void;

  onDisconnect(id: string): void;
}

export function ConnectionItem(props: ConnectionItemProps) {
  const offline = !props.item.connected;
  const color = !offline ? 'text.primary' : 'text.placeholder';

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
    <ListItem onClick={() => props.onActivate(props.item.id)}>
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
            actionIcon.action(props.item.id);
          }}
        >
          <actionIcon.Icon fontSize={12} />
        </ButtonIcon>
      </Flex>
    </ListItem>
  );
}
