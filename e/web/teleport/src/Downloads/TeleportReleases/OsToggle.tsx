import Box from 'design/Box';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import theme from 'design/theme';
import React from 'react';
import styled from 'styled-components';

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
  const buttons: OsButtonProps[] = [
    { icon: <Icons.Linux />, value: 'Linux' },
    { icon: <Icons.Apple />, value: 'macOS' },
    { icon: <Icons.Windows />, value: 'Windows' },
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
        backgroundColor: '#16204a',
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
                    backgroundColor: theme.colors.secondary.main,
                    color: 'white',
                  }
                : {}
            }
          >
            <StyledIconContainer>{button.icon}</StyledIconContainer>
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
  color: gray;
  cursor: pointer;
  display: flex;
  height: 36px;
  transition: all 0.3s;
  width: 90px;
`;

const StyledIconContainer = styled(Box)`
  height: 12px;
  margin-right: 8px;
  width: 12px;
  pointer-events: none;
`;
