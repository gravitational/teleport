import React from 'react';
import { ListItem } from 'teleterm/ui/Navigator/NavItem';
import { StatusIndicator } from './StatusIndicator';
import { Cross } from 'design/Icon';
import { ButtonIcon, Flex, Text } from 'design';
import { Connection } from 'teleterm/ui/services/connectionTracker';

export function ExpanderConnectionItem(props: ExpanderConnectionItemProps) {
  const offline = !props.item.connected;
  const color = !offline ? 'text.primary' : 'text.placeholder';

  return (
    <ListItem active={false} pl={5} onClick={() => props.onOpen(props.item.id)}>
      <StatusIndicator mr={2} connected={props.item.connected} />
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
        {offline && (
          <ButtonIcon
            color="text.placeholder"
            title="Remove"
            onClick={e => {
              e.stopPropagation();
              props.onRemove(props.item.id);
            }}
          >
            <Cross fontSize={12} />
          </ButtonIcon>
        )}
      </Flex>
    </ListItem>
  );
}

type ExpanderConnectionItemProps = {
  item: Connection;
  onOpen(id: string): void;
  onRemove(id: string): void;
};
