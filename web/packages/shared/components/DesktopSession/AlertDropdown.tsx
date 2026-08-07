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
import { BoxProps } from 'design/Box';
import { Cross, Warning } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import {
  ToastNotification,
  type ToastNotificationItem,
} from 'shared/components/ToastNotification';
import { useClickOutside } from 'shared/hooks/useClickOutside';
import { pluralize } from 'shared/utils/text';

export function AlertDropdown(
  props: {
    alerts: ToastNotificationItem[];
    onRemoveAlert: (id: string) => void;
  } & BoxProps
) {
  const { alerts, onRemoveAlert, ...boxProps } = props;
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
  useClickOutside(ref, () => setShowDropdown(false));

  return (
    // `display: contents` keeps this wrapper out of the layout while still
    // giving useClickOutside a node that contains both the toggle button and
    // the dropdown, so clicks on either are treated as "inside".
    <div ref={ref} style={{ display: 'contents' }}>
      <HoverTooltip
        tipContent={`${alerts.length} ${pluralize(alerts.length, 'alert')}`}
        placement="top"
      >
        <StyledButton px={2} onClick={toggleDropdown}>
          <Flex
            alignItems="center"
            justifyContent="space-between"
            color={theme.colors.text.main}
          >
            <Warning size={20} mr={2} /> {alerts.length}
          </Flex>
        </StyledButton>
      </HoverTooltip>
      {showDropdown && (
        <StyledCard
          mt={3}
          p={2}
          style={{
            maxHeight: window.innerHeight / 3,
          }}
          {...boxProps}
        >
          <Flex
            alignItems="center"
            justifyContent="space-between"
            pb={2}
            mb={1}
            borderBottom={1}
            borderColor="spotBackground.1"
          >
            <H3 px={3} style={{ overflow: 'visible' }}>
              {alerts.length} {pluralize(alerts.length, 'Alert')}
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
    </div>
  );
}

const StyledButton = styled(Button)`
  color: ${({ theme }) => theme.colors.light};
  min-height: 0;
  height: ${({ theme }) => theme.fontSizes[7] + 'px'};
  background-color: ${props =>
    props.theme.colors.interactive.solid.alert.default};
  &:hover,
  &:focus {
    background-color: ${props =>
      props.theme.colors.interactive.solid.alert.hover};
  }
`;

const StyledCard = styled(Card).attrs(props => ({
  right: 0,
  top: `${props.theme.fontSizes[7]}px`,
  ...props,
}))`
  display: flex;
  flex-direction: column;
  position: absolute;
  /* 320px notification width + horizontal card padding. */
  width: 336px;
  background-color: ${({ theme }) => theme.colors.levels.elevated};
`;

const StyledNotification = styled(ToastNotification)`
  background: ${({ theme }) => theme.colors.interactive.tonal.neutral[0]};
  box-shadow: none;
`;

const StyledOverflow = styled(Flex)`
  overflow-y: auto;
  overflow-x: hidden;
`;
