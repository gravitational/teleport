import React from 'react';
import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';
import { Add } from 'design/Icon';
import styled from 'styled-components';

interface AddNewClusterItemProps {
  index: number;

  onClick(): void;
}

export function AddNewClusterItem(props: AddNewClusterItemProps) {
  const { isActive } = useKeyboardArrowsNavigation({
    index: props.index,
    onRun: props.onClick,
  });

  return (
    <StyledListItem isActive={isActive} onClick={props.onClick}>
      <Add mr={1} color="inherit" />
      Add another cluster
    </StyledListItem>
  );
}

const StyledListItem = styled(ListItem)`
  justify-content: center;
  color: ${props => props.theme.colors.text.secondary};
`;
