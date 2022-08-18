import { SortAsc, SortDesc } from 'design/Icon';
import React, { forwardRef } from 'react';
import { Box } from 'design';
import styled from 'styled-components';

import { getUserWithClusterName } from 'teleterm/ui/utils';

import { UserIcon } from './UserIcon';
import { PamIcon } from './PamIcon';

interface IdentitySelectorProps {
  isOpened: boolean;
  userName: string;
  clusterName: string;

  onClick(): void;
  makeTitle: (userWithClusterName: string | undefined) => string;
}

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  IdentitySelectorProps
>((props, ref) => {
  const isSelected = props.userName && props.clusterName;
  const selectorText = isSelected && getUserWithClusterName(props);
  const Icon = props.isOpened ? SortAsc : SortDesc;
  const title = props.makeTitle(selectorText);

  return (
    <Container
      isOpened={props.isOpened}
      ref={ref}
      onClick={props.onClick}
      title={title}
    >
      {isSelected ? (
        <>
          <Box mr={2}>
            <UserIcon letter={props.userName[0]} />
          </Box>
        </>
      ) : (
        <PamIcon />
      )}
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
