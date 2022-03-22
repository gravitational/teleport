import React, { useState } from 'react';
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

  const title = props.userName
    ? `${props.userName}@${props.clusterName}`
    : props.clusterName;

  return (
    <ListItem
      onClick={props.onSelect}
      isActive={isActive}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <Flex justifyContent="space-between" alignItems="center" width="100%">
        <Text typography="body1" title={title}>
          {title}
        </Text>
        {props.isSelected ? (
          <Label kind="success" ml={2}>
            active
          </Label>
        ) : null}
        {props.isSyncing && <LinearProgress />}
        {isHovered && (
          <ButtonIcon
            mr="-10px"
            ml={1}
            color="text.placeholder"
            title="Logout"
            onClick={e => {
              e.stopPropagation();
              props.onLogout();
            }}
          >
            <CircleCross fontSize={12} />
          </ButtonIcon>
        )}
      </Flex>
    </ListItem>
  );
}
