import React from 'react';
import styled from 'styled-components';
import { ButtonPrimary } from 'design/Button';
import { Unlock } from 'design/Icon';
import Flex from 'design/Flex';
import Text from 'design/Text';
// import theme from 'design/theme';

export type Props = {
  children: React.ReactNode;
  [index: string]: any;
};
export function ButtonLockedFeature({ children, ...rest }) {
  return (
    <StyledButton onClick={() => console.log('TODO')} {...rest}>
      <Flex alignItems="flex-start">
        <Text>
          <UnlockIcon />
        </Text>
        {children}
      </Flex>
    </StyledButton>
  );
}

const StyledButton = styled(ButtonPrimary)(
  () => `
  text-transform: none;
  width: 100%;
  padding-top: 12px;
  padding-bottom: 12px;
  font-size: 12px;
  `
);

const UnlockIcon = styled(Unlock)(
  () => `
  color: inherit;
  font-weight: 500;
  font-size: 15px;
  margin-right: 4px;
  `
);
