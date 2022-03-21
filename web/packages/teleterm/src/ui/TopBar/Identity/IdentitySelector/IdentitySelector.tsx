import { SortAsc, SortDesc } from 'design/Icon';
import React, { forwardRef } from 'react';
import { Box, Text } from 'design';
import styled from 'styled-components';
import { UserIcon } from './UserIcon';

interface IdentitySelectorProps {
  isOpened: boolean;
  userName: string;
  hostName: string;

  onClick(): void;
}

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  IdentitySelectorProps
>((props, ref) => {
  const text =
    props.userName && props.hostName
      ? `${props.userName}@${props.hostName}`
      : 'Select Root Cluster';
  const Icon = props.isOpened ? SortAsc : SortDesc;

  return (
    <Container isOpened={props.isOpened} ref={ref} onClick={props.onClick}>
      {props.userName && (
        <Box mr={2}>
          <UserIcon letter={props.userName[0]} />
        </Box>
      )}
      <Text css={{ whiteSpace: 'nowrap' }} typography="subtitle1" title={text}>
        {text}
      </Text>
      <Icon ml={2} />
    </Container>
  );
});

const Container = styled.button`
  display: flex;
  font-family: inherit;
  background: inherit;
  cursor: pointer;
  align-items: center;
  color: ${props => props.theme.colors.text.primary};
  flex-direction: row;
  padding: 0 12px;
  height: 100%;
  max-width: 220px;
  border-radius: 4px;
  border-width: 1px;
  border-style: solid;
  border-color: ${props =>
    props.isOpened
      ? props.theme.colors.action.disabledBackground
      : 'transparent'};

  &:hover {
    background: ${props => props.theme.colors.primary.light};
  }
`;
