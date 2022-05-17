import { SortAsc, SortDesc } from 'design/Icon';
import React, { forwardRef } from 'react';
import { Box, Text } from 'design';
import styled from 'styled-components';
import { UserIcon } from './UserIcon';
import { PamIcon } from './PamIcon';
import { useKeyboardShortcutFormatters } from 'teleterm/ui/services/keyboardShortcuts';

interface IdentitySelectorProps {
  isOpened: boolean;
  userName: string;
  clusterName: string;

  onClick(): void;
}

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  IdentitySelectorProps
>((props, ref) => {
  const { getLabelWithShortcut } = useKeyboardShortcutFormatters();
  const isSelected = props.userName && props.clusterName;
  const selectorText = isSelected && `${props.userName}@${props.clusterName}`;
  const Icon = props.isOpened ? SortAsc : SortDesc;

  return (
    <Container
      isOpened={props.isOpened}
      ref={ref}
      onClick={props.onClick}
      title={getLabelWithShortcut(
        [selectorText, 'Open Profiles'].filter(Boolean).join('\n'),
        'toggle-identity'
      )}
    >
      {isSelected ? (
        <>
          <Box mr={2}>
            <UserIcon letter={props.userName[0]} />
          </Box>
          <Text style={{ whiteSpace: 'nowrap' }} typography="subtitle1">
            {selectorText}
          </Text>
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
  min-width: 0;
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
