import React from 'react';
import { Flex, ResourceIcon, Text } from 'design';
import styled from 'styled-components';

const StyledFlex = styled(Flex)`
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: 99px;
  width: max-content;
  padding: 0 8px 0 5px;
  height: 25px;
  color: ${p => p.theme.colors.text.slightlyMuted};
`;

export const OktaBadge = () => {
  return (
    <StyledFlex justifyContent="center" alignItems="center" gap={1}>
      <ResourceIcon name="okta" height="16px" />
      <Text typography="body3">Okta</Text>
    </StyledFlex>
  );
};
