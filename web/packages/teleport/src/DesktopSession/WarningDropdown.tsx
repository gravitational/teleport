/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useRef, useState } from 'react';
import { Text, Flex, Button, Card, ButtonIcon } from 'design';
import styled from 'styled-components';
import { Notification } from 'shared/components/Notification';
import { Warning, Close } from 'design/Icon';
import { orange } from 'design/theme/palette';
import { useClickOutside } from 'shared/hooks/useClickOutside';

import type { NotificationItem } from 'shared/components/Notification';

export function WarningDropdown({ warnings, onRemoveWarning }: Props) {
  const [showDropdown, setShowDropdown] = useState(false);
  const ref = useRef(null);

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
    <StyledRelative ref={ref}>
      <StyledButton
        title={'Warnings'}
        hasWarnings={warnings.length > 0}
        px={2}
        onClick={toggleDropdown}
      >
        <Flex alignItems="center" justifyContent="space-between">
          <StyledWarningIcon mr={2} /> {warnings.length}
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
              <Close />
            </ButtonIcon>
          </Flex>
          <StyledOverflow flexWrap="wrap" gap={2}>
            {warnings.map(warning => (
              <Notification
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
    </StyledRelative>
  );
}

const StyledWarningIcon = styled(Warning)`
  font-weight: ${({ theme }) => theme.fontWeights.bold};
  font-size: ${({ theme }) => theme.fontSizes[2] + 'px'};
  align-self: 'center';
`;

const StyledButton = styled(Button)`
  min-height: 0;
  height: ${({ theme }) => theme.fontSizes[7] + 'px'};
  background-color: ${props =>
    props.hasWarnings
      ? props.theme.colors.warning.main
      : props.theme.colors.action.disabled};
  &:hover,
  &:focus {
    background-color: ${props =>
      props.hasWarnings ? orange.A700 : props.theme.colors.action.disabled};
  }
`;

const StyledCard = styled(Card)`
  display: flex;
  flex-direction: column;
  position: absolute;
  right: 0;
  top: ${({ theme }) => theme.fontSizes[7] + 'px'};
  background-color: ${({ theme }) => theme.colors.levels.surfaceSecondary};
`;

const StyledRelative = styled.div`
  position: relative;
`;

const StyledOverflow = styled(Flex)`
  overflow-y: auto;
  overflow-x: hidden;
`;

type Props = {
  warnings: NotificationItem[];
  onRemoveWarning: (id: string) => void;
};
