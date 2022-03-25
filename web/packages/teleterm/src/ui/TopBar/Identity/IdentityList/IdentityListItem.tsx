import React, { useRef, useState } from 'react';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ButtonIcon, Flex, Label, Text } from 'design';
import LinearProgress from 'teleterm/ui/components/LinearProgress';
import { CircleCross } from 'design/Icon';

interface IdentityListItemProps {
  index: number;
  userName?: string;
  clusterName: string;
  isSelected: boolean;
  isSyncing: boolean;

  onSelect(): void;

  onLogout(): void;
}

export function IdentityListItem(props: IdentityListItemProps) {
  const [isHovered, setIsHovered] = useState(false);
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onSelect,
  });
  const ref = useRef<HTMLElement>();
  const [newWidth, setMaxWidth] = useState<number>();

  const title = props.userName
    ? `${props.userName}@${props.clusterName}`
    : props.clusterName;

  return (
    <ListItem
      onClick={props.onSelect}
      isActive={isActive}
      ref={ref}
      style={{ maxWidth: newWidth && newWidth + 'px' }}
      onMouseEnter={() => {
        // we set maxWidth to list item element, because otherwise it becomes wider on hover (new element is added to it)
        setMaxWidth(ref.current.clientWidth);
        setIsHovered(true);
      }}
      onMouseLeave={() => {
        setMaxWidth(undefined);
        setIsHovered(false);
      }}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        {props.isSyncing && <LinearProgress />}
        <Text typography="body1" title={title}>
          {title}
        </Text>
        <Flex alignItems="center">
          {props.isSelected ? (
            <Label kind="success" ml={2} style={{ height: 'fit-content' }}>
              active
            </Label>
          ) : null}
          {isHovered && (
            <ButtonIcon
              mr="-10px"
              ml={1}
              color="text.placeholder"
              title={`Log out from ${props.clusterName}`}
              onClick={e => {
                e.stopPropagation();
                props.onLogout();
              }}
            >
              <CircleCross fontSize={12} />
            </ButtonIcon>
          )}
        </Flex>
      </Flex>
    </ListItem>
  );
}
