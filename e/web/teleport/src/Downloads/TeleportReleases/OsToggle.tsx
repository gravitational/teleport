import React from 'react';
import styled, { useTheme } from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';

import type { OS } from 'e-teleport/services/downloads';

type OsToggleProps = {
  selectedOS: OS;
  onClick: (os: OS) => void;
};

type OsButtonProps = {
  icon: JSX.Element;
  value: OS;
};

export const OsToggle = ({ selectedOS, onClick }: OsToggleProps) => {
  const theme = useTheme();

  const buttons: OsButtonProps[] = [
    { icon: <Icons.Linux size="small" />, value: 'Linux' },
    { icon: <Icons.Apple size="small" />, value: 'macOS' },
    { icon: <Icons.Windows size="small" />, value: 'Windows' },
  ];

  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    const os = (event.target as HTMLButtonElement).value as OS;
    onClick(os);
  };

  return (
    <Flex
      aria-label="Choose operating system"
      role="group"
      p="1"
      borderRadius="8px"
      style={{
        backgroundColor: theme.colors.levels.surface,
        border: `${
          theme.type === 'light'
            ? `1px solid ${theme.colors.spotBackground[2]}`
            : ''
        }`,
      }}
    >
      {buttons.map(button => {
        const isSelected = selectedOS === button.value;
        return (
          <StyledButton
            // ensures screen readers can tab through and select OS options
            aria-pressed={isSelected}
            key={button.value}
            onClick={handleClick}
            value={button.value}
            style={
              isSelected
                ? {
                    backgroundColor: theme.colors.brand,
                    color: theme.colors.text.primaryInverse,
                  }
                : { color: theme.colors.text.main }
            }
          >
            <StyledIconContainer
              css={`
                .icon {
                  color: ${isSelected
                    ? theme.colors.text.primaryInverse
                    : theme.colors.text.main};
                }
              `}
            >
              {button.icon}
            </StyledIconContainer>
            {button.value}
          </StyledButton>
        );
      })}
    </Flex>
  );
};

const StyledButton = styled('button')`
  align-items: center;
  justify-content: center;
  background-color: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  height: 36px;
  transition: all 0.3s;
  width: 90px;

  &:hover {
    background: ${props => props.theme.colors.spotBackground[0]};
  }
`;

const StyledIconContainer = styled(Box)`
  height: 12px;
  margin-right: 8px;
  width: 12px;
  pointer-events: none;
`;
