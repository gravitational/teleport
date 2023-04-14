import React from 'react';
import styled from 'styled-components';
import Button from 'design/Button';
import { Unlock } from 'design/Icon';
import Flex from 'design/Flex';
import theme from 'design/theme';

export type Props = {
  children: React.ReactNode;
};
export function ButtonLockedFeature({ children }) {
  return (
    <StyledButton onClick={() => console.log('TODO')}>
      <Flex alignItems="flex-start">
        <UnlockIcon />
        {children}
      </Flex>
    </StyledButton>
  );
}

const StyledButton = styled(Button)(
  () => `
  text-transform: none;
  width: 100%;
  padding-top: 12px;
  padding-bottom: 12px;
  font-size: 12px;
  color: ${theme.colors.text.black};
  background-color: ${theme.colors.buttons.cta.default};
  `
);

const UnlockIcon = styled(Unlock)(
  () => `
  color: ${theme.colors.text.black};
  font-weight: 500;
  font-size: 15px;
  margin-right: 4px;
  `
);
