import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import React from 'react';

interface LogoutItemProps {
  index: number;

  onLogout(): void;
}

export function LogoutItem(props: LogoutItemProps) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onLogout,
  });

  return (
    <ListItem isActive={isActive} onClick={props.onLogout}>
      Logout
    </ListItem>
  );
}