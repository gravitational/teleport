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

import { useRef, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import { Button, ButtonIcon, Card, Flex, H3 } from 'design';
import { Cross, Warning } from 'design/Icon';
import {
  Notification,
  type NotificationItem,
} from 'shared/components/Notification';
import { useClickOutside } from 'shared/hooks/useClickOutside';

export function AlertDropdown({ alerts, onRemoveAlert }: Props) {
  const [showDropdown, setShowDropdown] = useState(false);
  const ref = useRef(null);
  const theme = useTheme();

  const toggleDropdown = () => {
    if (alerts.length > 0) {
      setShowDropdown(prevState => !prevState);
    }
  };

  // Dropdown is always closed if there are no errors to show
  if (alerts.length === 0 && showDropdown) setShowDropdown(false);

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
        title={'Alerts'}
        hasAlerts={alerts.length > 0}
        px={2}
        onClick={toggleDropdown}
      >
        <Flex
          alignItems="center"
          justifyContent="space-between"
          color={
            alerts.length ? theme.colors.text.main : theme.colors.text.disabled
          }
        >
          <Warning size={20} mr={2} /> {alerts.length}
        </Flex>
      </StyledButton>
      {showDropdown && (
        <StyledCard
          mt={3}
          p={2}
          style={{
            maxHeight: window.innerHeight / 3,
          }}
        >
          <Flex alignItems="center" justifyContent="space-between">
            <H3 px={3} style={{ overflow: 'visible' }}>
              {alerts.length} {alerts.length > 1 ? 'Alerts' : 'Alert'}
            </H3>
            <ButtonIcon size={1} ml={1} mr={2} onClick={toggleDropdown}>
              <Cross size="medium" />
            </ButtonIcon>
          </Flex>
          <StyledOverflow flexDirection="column" gap={2}>
            {alerts.map(alert => (
              <StyledNotification
                key={alert.id}
                item={alert}
                onRemove={() => onRemoveAlert(alert.id)}
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
    props.hasAlerts
      ? props.theme.colors.warning.main
      : props.theme.colors.spotBackground[1]};
  &:hover,
  &:focus {
    background-color: ${props =>
      props.hasAlerts
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
  alerts: NotificationItem[];
  onRemoveAlert: (id: string) => void;
};
