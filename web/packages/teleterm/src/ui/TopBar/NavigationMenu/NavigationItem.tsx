import React from 'react';
import styled from 'styled-components';

import { Box, Text, Flex } from 'design';

export function NavigationItem({ item, closeMenu }: NavigationItemProps) {
  const handleClick = () => {
    item.onNavigate();
    closeMenu();
  };

  return (
    <ListItem p={2} pl={0} gap={2} onClick={handleClick}>
      <IconBox p={2}>{item.Icon}</IconBox>
      <Text>{item.title}</Text>
    </ListItem>
  );
}

const ListItem = styled(Flex)`
  border-radius: 4px;
  align-items: center;
  justify-content: start;
  cursor: pointer;
  &:hover {
    background: ${props => props.theme.colors.primary.lighter};
  }
`;

const IconBox = styled(Box)`
  display: flex;
  align-items: center;
  border-radius: 4px;
  background-color: ${props => props.theme.colors.primary.lighter};
`;

type NavigationItemProps = {
  item: {
    title: string;
    Icon: JSX.Element;
    onNavigate: () => void;
  };
  closeMenu: () => void;
};
