import React from 'react';

import { Add } from 'design/Icon';

import styled from 'styled-components';

import { useKeyboardArrowsNavigation } from 'teleterm/ui/components/KeyboardArrowsNavigation';
import { ListItem } from 'teleterm/ui/components/ListItem';

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
  border-radius: 0;
  height: 38px;
  justify-content: center;
  color: ${props => props.theme.colors.text.secondary};
`;
