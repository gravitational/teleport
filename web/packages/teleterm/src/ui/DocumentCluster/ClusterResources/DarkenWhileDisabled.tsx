import React from 'react';
import styled from 'styled-components';
import { Box } from 'design';

export const DarkenWhileDisabled: React.FC<Props> = ({
  children,
  disabled,
}) => (
  <DarkenWhileDisabledContainer className={disabled ? 'disabled' : ''}>
    {children}
  </DarkenWhileDisabledContainer>
);

const DarkenWhileDisabledContainer = styled(Box)`
  // The timing functions of transitions have been chosen so that the element loses opacity slowly
  // when entering the disabled state but gains it quickly when going out of the disabled state.
  transition: opacity 150ms ease-out;
  &.disabled {
    pointer-events: none;
    opacity: 0.7;
    transition: opacity 150ms ease-in;
  }
`;

type Props = {
  disabled: boolean;
};
