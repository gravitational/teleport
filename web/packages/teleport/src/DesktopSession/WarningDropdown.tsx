/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import React, { useRef, useState } from 'react';
import { Text, Flex, Button, Card, ButtonIcon } from 'design';
import styled, { useTheme } from 'styled-components';
import { Notification } from 'shared/components/Notification';
import { Warning, Cross } from 'design/Icon';
import { useClickOutside } from 'shared/hooks/useClickOutside';

import type { NotificationItem } from 'shared/components/Notification';

export function WarningDropdown({ warnings, onRemoveWarning }: Props) {
  const [showDropdown, setShowDropdown] = useState(false);
  const ref = useRef(null);
  const theme = useTheme();

  const toggleDropdown = () => {
    if (warnings.length > 0) {
      setShowDropdown(prevState => !prevState);
    }
  };

  // Dropdown is always closed if there are no errors to show
  if (warnings.length === 0 && showDropdown) setShowDropdown(false);

  // Close the dropdown if it's open and the user clicks outside of it
  useClickOutside(ref, () => {
    setShowDropdown(prevState => {
      if (prevState) {
        return false;
      }
    });
  });

  return (
    <>
      <StyledButton
        title={'Warnings'}
        hasWarnings={warnings.length > 0}
        px={2}
        onClick={toggleDropdown}
      >
        <Flex
          alignItems="center"
          justifyContent="space-between"
          color={
            warnings.length
              ? theme.colors.text.main
              : theme.colors.text.disabled
          }
        >
          <Warning size={20} mr={2} /> {warnings.length}
        </Flex>
      </StyledButton>
      {showDropdown && (
        <StyledCard
          mt={3}
          p={2}
          style={{
            maxHeight: window.innerHeight / 4,
          }}
        >
          <Flex alignItems="center" justifyContent="space-between">
            <Text typography="h6" px={3} style={{ overflow: 'visible' }}>
              {warnings.length} {warnings.length > 1 ? 'Warnings' : 'Warning'}
            </Text>
            <ButtonIcon size={1} ml={1} mr={2} onClick={toggleDropdown}>
              <Cross size="medium" />
            </ButtonIcon>
          </Flex>
          <StyledOverflow flexWrap="wrap" gap={2}>
            {warnings.map(warning => (
              <StyledNotification
                key={warning.id}
                item={warning}
                onRemove={() => onRemoveWarning(warning.id)}
                Icon={Warning}
                getColor={theme => theme.colors.warning.main}
                isAutoRemovable={false}
              />
            ))}
          </StyledOverflow>
        </StyledCard>
      )}
    </>
  );
}

const StyledButton = styled(Button)`
  color: ${({ theme }) => theme.colors.light};
  min-height: 0;
  height: ${({ theme }) => theme.fontSizes[7] + 'px'};
  background-color: ${props =>
    props.hasWarnings
      ? props.theme.colors.warning.main
      : props.theme.colors.spotBackground[1]};
  &:hover,
  &:focus {
    background-color: ${props =>
      props.hasWarnings
        ? props.theme.colors.warning.hover
        : props.theme.colors.spotBackground[2]};
  }
`;

const StyledCard = styled(Card)`
  display: flex;
  flex-direction: column;
  position: absolute;
  right: 0;
  top: ${({ theme }) => theme.fontSizes[7] + 'px'};
  background-color: ${({ theme }) => theme.colors.levels.elevated};
`;

const StyledNotification = styled(Notification)`
  background: ${({ theme }) => theme.colors.spotBackground[0]};
  ${({ theme }) =>
    theme.type === 'light' && `border: 1px solid ${theme.colors.text.muted};`}
  box-shadow: none;
`;

const StyledOverflow = styled(Flex)`
  overflow-y: auto;
  overflow-x: hidden;
`;

type Props = {
  warnings: NotificationItem[];
  onRemoveWarning: (id: string) => void;
};
